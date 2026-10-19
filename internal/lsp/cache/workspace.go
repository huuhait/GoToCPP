package cache

import (
	"context"
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"sync"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/memoize"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// What the workspace-wide features (step 11.8: references, rename) need of
// the pipeline: the imports of a package without checking it, the package of
// an import path in the types of a checked package, and the check of a
// package with its test files.

// An ImportRef is an import of a package, as the loader resolved it.
type ImportRef struct {
	ImportPath string
	Dir        string // the folder of the package (in Config.StdlibDir for a standard package)
	Std        bool
}

// ImportsOf returns the imports of the package in the folder dir (an
// absolute name) in the snapshot: its metadata, which is cheap (the headers
// of its files and the resolution of their imports) and comes from the
// cache, so it checks nothing. The imports are in the order that the loader
// asked for them.
func (s *Snapshot) ImportsOf(ctx context.Context, dir string) ([]ImportRef, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	h, err := st.handle(ctx, pkgID{dir: filepath.Clean(dir)}, "", nil)
	if err != nil {
		return nil, err
	}
	refs := make([]ImportRef, len(h.meta.deps))
	for i, d := range h.meta.deps {
		refs[i] = ImportRef{ImportPath: d.importPath, Dir: d.dir, Std: d.std}
	}
	return refs, nil
}

// Lookup returns the package with the import path path as the types of the
// check know it: the package itself, or one that the check read from export
// data (its imports, and what their types mention, directly or not), or nil.
// An object of another check is found in it by its objectpath.
func (pkg *Package) Lookup(path string) *types.Package {
	if pkg.Check != nil && pkg.Check.Types != nil && pkg.Check.Types.Path() == path {
		return pkg.Check.Types
	}
	return pkg.imported[path]
}

// ImportedPaths returns the import paths of the packages that the check read
// from export data: its imports, and the packages their types mention, which
// Lookup finds. It is sorted.
func (pkg *Package) ImportedPaths() []string {
	paths := make([]string, 0, len(pkg.imported))
	for p := range pkg.imported {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// A TestCheck is the check of a package together with its _test.gp files, in
// the import graph of its test (load.Config.Tests): Program.Roots are the
// package with its internal tests and, if it has any, its external test
// package; their dependencies come from source, and none of it is cached.
type TestCheck struct {
	Program *load.Program
	Result  *check.Result
}

// CheckTests loads and checks the package in the folder dir with its test
// files, from the files of the snapshot. It costs a load of the package and
// all of its dependencies from source, which no request that must be fast
// may do: the features that need the test files (references and rename) do it
// only for a package whose test files mention the name they look for. The
// type information is partial where the code has errors
// (check.Config.Partial).
func (s *Snapshot) CheckTests(ctx context.Context, dir string) (*TestCheck, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	dir = filepath.Clean(dir)
	// One check per folder and snapshot: the cursor of an editor moves
	// without changing anything, and a highlight at each place would check
	// the same tests again and again. (An edit makes a new state, which
	// has none.)
	st.mu.Lock()
	if st.released {
		st.mu.Unlock()
		return nil, errReleased
	}
	pr := st.tests[dir]
	if pr == nil {
		pr = memoize.NewPromise(func(ctx context.Context) (*TestCheck, error) { return st.checkTests(ctx, dir) })
		if st.tests == nil {
			st.tests = make(map[string]*memoize.Promise[*TestCheck])
		}
		st.tests[dir] = pr
	}
	st.mu.Unlock()
	return pr.Get(ctx)
}

// checkTests is CheckTests without the memo.
func (st *state) checkTests(ctx context.Context, dir string) (*TestCheck, error) {
	p := st.p
	h, err := st.handle(ctx, pkgID{dir: dir}, "", nil)
	if err != nil {
		return nil, err
	}
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-p.sem }()
	p.stats.testChecks.Add(1)

	pattern, loadDir := h.loadPattern()
	cfg := p.loadConfig(h, loadDir, false)
	cfg.Fset = token.NewFileSet()
	cfg.Tests = true
	cfg.FS = stateFS{s: st} // the dependencies read the snapshot's files, the open ones too
	prog := load.Load(cfg, pattern)
	files := 0
	for _, root := range prog.Roots {
		files += len(root.Files)
	}
	if files == 0 {
		return nil, fmt.Errorf("cache: no .gp files in %s", dir)
	}
	rec := &probeRecord{pr: p.prober}
	r := check.Check(prog, check.Config{
		Phase:     p.cfg.Phase,
		GOOS:      p.cfg.GOOS,
		GOARCH:    p.cfg.GOARCH,
		Context:   ctx,
		CXX:       p.prober.findCXX(rec),
		Probe:     p.prober.probeFor(rec),
		PkgConfig: p.prober.pkgConfigFor(rec),
		CHeader:   p.prober.cheaderFor(rec),
		Partial:   true,

		NoDerivedBodies: true, // nothing here lowers the program
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &TestCheck{Program: prog, Result: r}, nil
}

// Edited returns a snapshot that has the open documents of s, with the
// texts of texts instead (a document that is not open gets one): what the
// workspace would be after an edit that the user has not made yet. Rename
// checks its result with it before it answers. The snapshot belongs to a
// session of its own, which shares the pipeline's caches, so only what
// differs is parsed and checked; the session is not the server's, so
// nothing is published for it. release must be called when the snapshot is
// no longer needed.
func (s *Snapshot) Edited(texts map[protocol.DocumentURI]string) (*Snapshot, func(), error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, nil, errNoPipeline
	}
	// The snapshot is made from s as an edit of the open documents is: its
	// state is a clone of s's (State.Clone), so what the edit leaves alone (the
	// files read, the packages checked) is not read or checked again, and only
	// the packages that the texts change, and the ones that import them, are.
	sess := &Session{live: 1}
	overlays := make(map[protocol.DocumentURI]*Overlay, len(s.overlays)+len(texts))
	for uri, o := range s.overlays {
		overlays[uri] = o
	}
	changed := make(map[protocol.DocumentURI]bool, len(texts))
	for uri, text := range texts {
		item := protocol.TextDocumentItem{URI: uri, LanguageID: "gocpp", Version: 1, Text: text}
		if o := s.Overlay(uri); o != nil {
			item.LanguageID, item.Version = o.languageID, o.version
		}
		overlays[uri] = newOverlay(item)
		changed[uri] = true
	}
	first := newSnapshot(sess, 0, overlays, nil)
	first.state = st.Clone(first, Change{Files: changed})
	sess.current = first
	snap, release := sess.Snapshot()
	var once sync.Once
	return snap, func() {
		once.Do(func() {
			release()
			sess.Shutdown()
			// The session's own reference: nobody else will change it.
			sess.mu.Lock()
			cur := sess.current
			sess.mu.Unlock()
			cur.decref()
		})
	}, nil
}
