package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/tools/go/gcexportdata"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/memoize"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// How a package gets checked, in two stages:
//
//  1. Its handle (per snapshot, cheap): its files and their hashes, and its
//     metadata (the packages it imports, see loadMetadata), kept by the
//     hash of the headers of its files, so an edit in a function body
//     needs no new metadata, and valid while the reads that resolved its
//     imports give the same results (metadata.valid). The handles of its
//     imports are made first,
//     so the handles never form a cycle: an import of a package whose
//     handle is being made is cut, and the packages of the cycle, and
//     those that import them, are "cyclic" (see handle.cyclic).
//  2. Its key and its check (shared by the snapshots, costly): the key is
//     the hash of its files, of the reads of its metadata and of the API
//     hashes of its imports, which come from their summaries (checked
//     first, at the same time). The
//     check of a key runs once; the summary of a package is what its
//     importers need, and stays longer than the whole check.
//
// Stage 2 waits only for the stage 2 of the imports, so it can't deadlock.

// A pkgID names a package: by its folder, or for a standard package by
// its import path (dir is its folder in Config.StdlibDir then).
type pkgID struct {
	dir string
	std bool
}

// A handle is a package as one snapshot (and the next ones, while the
// package doesn't change) sees it: stage 1.
type handle struct {
	id         pkgID
	importPath string // for a standard package; the others get theirs from the metadata
	files      []*fileHandle
	mod        *fileHandle // the gocpp.mod file, or nil
	localKey   Hash        // the hash of the files and the configuration
	meta       *metadata
	deps       []*handle // the handles of the imports of meta, without those that make a cycle
	cut        []pkgID   // the imports that make a cycle

	// cyclic is set when the package is in an import cycle or imports
	// such a package: then cut is not empty or a dep is cyclic. Its check
	// loads the cyclic imports from source (the loader reports the
	// cycle, as gocpp check does), with the files of the snapshot that
	// made the handle (fs), and it is kept in own, not in the caches by
	// key: the key would not have the files of the cut imports.
	cyclic bool
	fs     stateFS // keeps the state that made the handle, and its maps, alive while the handle lives
	own    *memoize.Promise[*Package]

	// open is set when a file of the package is an open document. Then
	// last keeps its checked package while the handle lives (the
	// snapshots that hold it), so the LRU of the check cache never drops
	// what the user is looking at (README §2.2 item 10).
	open bool
	last atomic.Pointer[Package]

	// keyed is stage 2: the key, from the summaries of deps.
	keyed *memoize.Promise[*keyed]
}

// A metadata is what a package imports, as the loader resolves it: from
// the headers of the files (headerEnd) and the reads of the resolution.
type metadata struct {
	deps   []depRef // the imports, in the order the loader asked for them
	reads  []*read  // what the load read through load.Config.FS
	digest Hash     // the hash of the reads
}

// valid reports whether the reads of m give the same results in the
// snapshot of s: then the imports resolve as they did.
func (m *metadata) valid(s *state) bool {
	for _, r := range m.reads {
		cur, err := s.do(r.kind, r.path)
		if err != nil {
			return false
		}
		a, b := newHasher("read"), newHasher("read")
		r.digest(a)
		cur.digest(b)
		if a.sum() != b.sum() {
			return false
		}
	}
	return true
}

// A depRef is an import of a package, as the loader resolved it.
type depRef struct {
	importPath string
	dir        string
	std        bool
}

// keyed is the key of a package's check and the summaries of its imports
// (in the order of handle.deps).
type keyed struct {
	key  Hash
	deps []*summary
}

// A summary is what the packages that import a package need of it: the
// loader's summary, its export data, the side table of its opaque types,
// and its API hash. It comes from a check, or from the disk cache
// (disk.go).
type summary struct {
	importPath string
	load       *load.Package // load.Package.Summary
	export     []byte        // gcexportdata; nil if it was not type-checked
	hasC       bool          // check.Dep.HasC
	failed     bool          // check.Dep.Failed
	apiHash    Hash

	// opaque is the side table of the opaque types of the package
	// (opaque.go), and tables are the non-empty tables of the package and
	// of the packages it imports, directly or not, by import path.
	opaque []opaqueEntry
	tables map[string][]opaqueEntry

	// records are the probe records of the check of the package and of
	// the packages it imports, directly or not, that asked for the C++
	// compiler or a probe (probe.go): the summary is stale when one of
	// them is. own is the record of the package's own check (or of its
	// summary from disk, see prober.replay), if it asked.
	records []*probeRecord
	own     *probeRecord

	// exp is the decoded export data, for Snapshot.Exports (once).
	expOnce sync.Once
	exp     *Exports
	expErr  error
}

// inherit adds what s has of the summaries of its imports deps (nil for a
// cyclic one): their probe records and their side tables.
func (s *summary) inherit(deps []*summary) {
	for _, d := range deps {
		if d != nil {
			for _, r := range d.records {
				if !slices.Contains(s.records, r) {
					s.records = append(s.records, r)
				}
			}
		}
	}
	s.tables = mergeTables(s.importPath, s.opaque, deps)
}

// stale reports whether the prober would answer a check of the package, or
// of one it imports, differently now.
func (s *summary) stale() bool {
	for _, r := range s.records {
		if r.stale() {
			return true
		}
	}
	return false
}

// A Package is a package of a snapshot after the check: loaded alone, with
// the packages it imports from their summaries. It is shared by every
// snapshot in which the package and the API of its imports are the same,
// so it must never be changed.
type Package struct {
	// Program is the load of the package: Program.Roots[0] is the
	// package, and Program.Packages holds only it (the imports are
	// summaries, see load.Config.Deps), unless the loader had to load an
	// import itself (an import cycle).
	Program *load.Program

	// Check is the checked package: its types, types.Info and
	// diagnostics (check.Package). Its Errors are all the diagnostics of
	// the package, but those of Program.Errors.
	Check *check.Package

	// APIHash is the hash of what the importers see of the package (see
	// apiHash).
	APIHash Hash

	// cost is the cost of the package in the check cache: an estimate of
	// its memory, from the size of its files (sourceFactor) and of the
	// export data of its imports, whose types it holds (exportFactor).
	cost    int64
	summary *summary

	// For OpaqueType (opaque.go): the content of the package's files by
	// name, the packages read from export data by import path, and the
	// side tables of the imports; depTypes is made from them once.
	src       map[string][]byte
	imported  map[string]*types.Package
	tables    map[string][]opaqueEntry
	depOnce   sync.Once
	depTypes  map[types.Object]string
	funcOnce  sync.Once
	funcTypes map[types.Object]string // the functions of the package (funcSpellings)
}

// Fset returns the FileSet of the package's files and of the objects of the
// packages it imports (from their export data).
func (pkg *Package) Fset() *token.FileSet { return pkg.Program.Fset }

// Provisional reports whether the check of the package, or of a package
// that it imports, directly or not, went on without an answer of the
// namespace probe that runs in the background (probe.go): the uses of the
// names it didn't know yet are opaque, without errors, and the include
// errors are missing. When the answers come, the package is Stale.
func (pkg *Package) Provisional() bool {
	for _, r := range pkg.summary.records {
		if r.provisional() {
			return true
		}
	}
	return false
}

// Stale reports whether the namespace probe answers the check of the
// package, or of a package it imports, differently now than when it was
// checked: a new snapshot (Session.Recheck of its folder) checks it
// again. Config.OnProbe says when this may have changed.
func (pkg *Package) Stale() bool { return pkg.summary.stale() }

// Package returns the checked package of the folder dir (an absolute name)
// in the snapshot. Only the package and the imports whose API changed are
// checked again after an edit; the rest comes from the caches. It is an
// error when the session has no Pipeline (NewSession with its NewState).
func (s *Snapshot) Package(ctx context.Context, dir string) (*Package, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	h, err := st.handle(ctx, pkgID{dir: filepath.Clean(dir)}, "", nil)
	if err != nil {
		return nil, err
	}
	return st.p.packageOf(ctx, h)
}

// PackageOf returns the checked package of the file uri: the package of its
// folder (Snapshot.Package).
func (s *Snapshot) PackageOf(ctx context.Context, uri protocol.DocumentURI) (*Package, error) {
	path := uri.Path()
	if path == "" {
		return nil, fmt.Errorf("%s is not a file", uri)
	}
	return s.Package(ctx, filepath.Dir(path))
}

// ParseFile returns the file uri of the snapshot (its overlay, or else the
// file on disk) parsed in the mode mode (ParseFull for the features), from
// the parse cache. The ParsedFile must not be changed.
func (s *Snapshot) ParseFile(ctx context.Context, uri protocol.DocumentURI, mode parser.Mode) (*ParsedFile, error) {
	path := uri.Path()
	if path == "" {
		return nil, fmt.Errorf("%s is not a file", uri)
	}
	return s.ParsePath(ctx, path, mode)
}

// ParsePath is ParseFile for the file with the absolute name path, which
// can also be a file of the standard library (a name in
// Config.StdlibDir, as the positions of its objects have it).
func (s *Snapshot) ParsePath(ctx context.Context, path string, mode parser.Mode) (*ParsedFile, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	f, err := st.fileOrStd(path)
	if err != nil {
		return nil, err
	}
	return st.p.parse(ctx, f, mode)
}

// ReadFile returns the content of the file with the absolute name path in
// the snapshot: its overlay, or else the file on disk as the snapshot read
// it (once): the text that the snapshot's checks saw. A name in
// Config.StdlibDir is a file of the standard library.
func (s *Snapshot) ReadFile(path string) ([]byte, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	f, err := st.fileOrStd(path)
	if err != nil {
		return nil, err
	}
	return f.src, f.err
}

// PackageFiles returns the absolute names of the .gp files of the package
// lp (a package of a checked program, or the summary of one that it
// imports) in the snapshot, sorted, with the test files: the files of its
// folder, or of the standard package.
func (s *Snapshot) PackageFiles(lp *load.Package) ([]string, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	var names []string
	if lp.Std {
		for _, f := range st.p.stdFiles(lp.ImportPath) {
			names = append(names, f.path)
		}
		return names, nil
	}
	d, err := st.dir(filepath.Clean(lp.Dir))
	if err != nil {
		return nil, err
	}
	for _, name := range d.names {
		names = append(names, filepath.Join(lp.Dir, name))
	}
	return names, nil
}

// IsStdFile reports whether path is the name of a file of the standard
// library (in Config.StdlibDir): it has no file on disk.
func (s *Snapshot) IsStdFile(path string) bool {
	st, ok := s.state.(*state)
	return ok && st.p.inStdlib(path)
}

// inStdlib reports whether path is in Config.StdlibDir.
func (p *Pipeline) inStdlib(path string) bool {
	return strings.HasPrefix(path, p.cfg.StdlibDir+string(filepath.Separator))
}

// fileOrStd returns the file path of the snapshot (state.file), or the file
// of the standard library with that name.
func (s *state) fileOrStd(path string) (*fileHandle, error) {
	if !s.p.inStdlib(path) {
		return s.file(path)
	}
	rel := strings.TrimPrefix(path, s.p.cfg.StdlibDir+string(filepath.Separator))
	for _, f := range s.p.stdFiles(filepath.ToSlash(filepath.Dir(rel))) {
		if f.path == path {
			return f, nil
		}
	}
	return &fileHandle{path: path, err: fs.ErrNotExist}, nil
}

var errNoPipeline = errors.New("cache: the session has no pipeline")

// handle returns the handle of the package id (stage 1). importPath is the
// import path of a standard package. stack holds the packages whose handles
// are being made, which import this one: an import of one of them is a
// cycle, which is cut.
func (s *state) handle(ctx context.Context, id pkgID, importPath string, stack []pkgID) (*handle, error) {
	s.mu.Lock()
	if s.released {
		s.mu.Unlock()
		return nil, errReleased
	}
	h := s.handles[id]
	s.mu.Unlock()
	if h != nil {
		return h, nil
	}

	h = &handle{id: id, importPath: importPath}
	var err error
	if id.std {
		h.files = s.p.stdFiles(importPath)
	} else {
		var d *dirHandle
		if d, err = s.dir(id.dir); err != nil {
			return nil, err
		}
		for _, name := range d.names {
			f, err := s.file(filepath.Join(id.dir, name))
			if err != nil {
				return nil, err
			}
			h.files = append(h.files, f)
		}
		if h.mod, err = s.modFile(id.dir); err != nil {
			return nil, err
		}
	}

	// The local key: the configuration, the package and its files. The
	// key of the metadata has only the headers of the files.
	local, imports := newHasher("local"), newHasher("imports")
	for _, x := range []*hasher{local, imports} {
		x.hash(s.p.cfgKey)
		x.str(id.dir)
		x.bool(id.std)
		x.str(importPath)
		if h.mod != nil {
			x.str(h.mod.path)
			x.hash(h.mod.hash)
		}
		x.int(int64(len(h.files)))
	}
	for _, f := range h.files {
		local.str(f.path)
		local.hash(f.hash)
		imports.str(f.path)
		if id.std {
			imports.hash(f.hash) // read once anyway (loadMetadata)
			continue
		}
		pf, err := s.p.parse(ctx, f, parser.ImportsOnly)
		if err != nil {
			imports.str(err.Error())
			continue
		}
		imports.hash(hashOf(pf.Src)) // the header
	}
	h.localKey = local.sum()
	for _, f := range h.files {
		if s.overlays[f.path] != nil {
			h.open = true
		}
	}
	p := s.p
	h.fs = stateFS{s: s}
	if h.meta, err = p.metadataFor(h, s, imports.sum()); err != nil {
		return nil, err
	}

	// The handles of the imports, unless they close a cycle.
	stack = append(stack, id)
	for _, ref := range h.meta.deps {
		depID := pkgID{dir: ref.dir, std: ref.std}
		if slices.Contains(stack, depID) {
			h.cut = append(h.cut, depID)
			h.cyclic = true
			continue
		}
		depPath := ""
		if ref.std {
			depPath = ref.importPath
		}
		dep, err := s.handle(ctx, depID, depPath, stack)
		if err != nil {
			return nil, err
		}
		h.deps = append(h.deps, dep)
		h.cyclic = h.cyclic || dep.cyclic
	}
	h.keyed = memoize.NewPromise(func(ctx context.Context) (*keyed, error) { return p.keyOf(ctx, h) })
	if !h.cyclic {
		h.fs = stateFS{} // only a cyclic check reads the snapshot
	} else {
		h.own = memoize.NewPromise(func(ctx context.Context) (*Package, error) {
			k, err := h.keyed.Get(ctx)
			if err != nil {
				return nil, err
			}
			return p.check(ctx, h, k)
		})
	}
	return store(s, handlesOf, id, h)
}

// loadPattern returns the pattern of load.Load for the package of h, and
// the folder to load it in.
func (h *handle) loadPattern() (pattern, dir string) {
	if h.id.std {
		return h.importPath, string(filepath.Separator)
	}
	return h.id.dir, h.id.dir
}

// loadConfig returns the load.Config of the pipeline, with the files of h
// (or, with headers, their headers only) in the overlay.
func (p *Pipeline) loadConfig(h *handle, dir string, headers bool) load.Config {
	cfg := load.Config{
		Dir:       dir,
		Stdlib:    p.cfg.Stdlib,
		StdlibDir: p.cfg.StdlibDir,
		GoStd:     p.cfg.GoStd,
		GOOS:      p.cfg.GOOS,
		GOARCH:    p.cfg.GOARCH,
		Tags:      p.cfg.Tags,
		AllErrors: true,
	}
	if h.id.std {
		return cfg // the standard library has no overlay
	}
	cfg.Overlay = make(map[string][]byte, len(h.files)+1)
	for _, f := range h.files {
		if f.err != nil {
			continue // the loader reads it and reports the error
		}
		src := f.src
		if headers {
			if pf, _, done := p.parses.Peek(parseKey{f.path, f.hash, parser.ImportsOnly}); done && pf != nil {
				src = pf.Src // the header
			}
		}
		cfg.Overlay[f.path] = src
	}
	if h.mod != nil {
		cfg.Overlay[h.mod.path] = h.mod.src
	}
	return cfg
}

// metadataFor returns the metadata of h in the snapshot of s: the one kept
// for the hash of its headers if it is valid there, or else a new one.
func (p *Pipeline) metadataFor(h *handle, s *state, importsKey Hash) (*metadata, error) {
	p.metaMu.Lock()
	m := p.metas[importsKey]
	p.metaMu.Unlock()
	if m != nil && m.valid(s) {
		return m, nil
	}
	m, err := p.loadMetadata(h, s)
	if err != nil {
		return nil, err
	}
	p.metaMu.Lock()
	if len(p.metas) >= maxMetadata {
		clear(p.metas) // a metadata is cheap to make again
	}
	p.metas[importsKey] = m
	p.metaMu.Unlock()
	return m, nil
}

// loadMetadata loads the package of h with only the headers of its files,
// and its imports as empty summaries, to find out what it imports: the
// loader resolves the import paths of the headers (load.Config.Deps sees
// each one), with the files and folders of the snapshot of s, and records
// what it reads. The standard library is not read through the FS: it never
// changes.
func (p *Pipeline) loadMetadata(h *handle, s *state) (*metadata, error) {
	p.stats.metadata.Add(1)
	pattern, dir := h.loadPattern()
	cfg := p.loadConfig(h, dir, true)
	m := &metadata{}
	var fsErr error
	cfg.FS = recordingFS{stateFS{s: s, log: &m.reads}, &fsErr}
	cfg.Deps = func(importPath, dir string, std bool) *load.Package {
		m.deps = append(m.deps, depRef{importPath, dir, std})
		return (&load.Package{ImportPath: importPath, Dir: dir, Std: std, Errors: new(diag.List)}).Summary()
	}
	load.Load(cfg, pattern)
	if fsErr != nil {
		return nil, fsErr
	}
	x := newHasher("reads")
	for _, r := range m.reads {
		r.digest(x)
	}
	m.digest = x.sum()
	return m, nil
}

// A recordingFS is a stateFS that keeps the first error of the state
// itself (not of a file), which the load would take for a missing file.
type recordingFS struct {
	stateFS
	err *error
}

func (fsys recordingFS) keep(err error) error {
	if err != nil && *fsys.err == nil {
		*fsys.err = err
	}
	return err
}

func (fsys recordingFS) ReadFile(name string) ([]byte, error) {
	r, err := fsys.get('f', name)
	if fsys.keep(err) != nil {
		return nil, err
	}
	return r.file()
}

func (fsys recordingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	r, err := fsys.get('d', name)
	if fsys.keep(err) != nil {
		return nil, err
	}
	return r.dir()
}

func (fsys recordingFS) Stat(name string) (fs.FileInfo, error) {
	r, err := fsys.get('s', name)
	if fsys.keep(err) != nil {
		return nil, err
	}
	return r.stat()
}

// keyOf computes the key of h (stage 2): it waits for the summaries of the
// imports, all at the same time. (A cyclic import is loaded from source,
// so its summary is not needed: k.deps has nil for it.)
func (p *Pipeline) keyOf(ctx context.Context, h *handle) (*keyed, error) {
	k := &keyed{deps: make([]*summary, len(h.deps))}
	errs := make([]error, len(h.deps))
	var wg sync.WaitGroup
	for i, dep := range h.deps {
		if dep.cyclic {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			k.deps[i], errs[i] = p.summaryOf(ctx, dep)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	x := newHasher("check")
	x.hash(h.localKey)
	x.hash(h.meta.digest)
	x.int(int64(len(h.deps)))
	for i, dep := range h.deps {
		x.str(dep.id.dir)
		x.bool(dep.id.std)
		if s := k.deps[i]; s != nil {
			x.str(s.importPath)
			x.hash(s.apiHash)
		}
	}
	k.key = x.sum()
	return k, nil
}

// summaryOf returns the summary of h: from the cache, from the disk cache
// (unless a checked package of the key is in memory), or after its check,
// which is then written to the disk cache.
func (p *Pipeline) summaryOf(ctx context.Context, h *handle) (*summary, error) {
	if h.cyclic {
		pkg, err := h.own.Get(ctx)
		if err != nil {
			return nil, err
		}
		return pkg.summary, nil
	}
	k, err := h.keyed.Get(ctx)
	if err != nil {
		return nil, err
	}
	fromDisk := true
	for {
		s, err := p.summaries.Get(ctx, k.key, func(ctx context.Context) (*summary, error) {
			if pkg, _, ok := p.checks.Peek(k.key); ok && pkg != nil {
				p.saveSummary(h, k, pkg.summary)
				return pkg.summary, nil
			}
			if fromDisk {
				if s := p.diskSummaryOf(h, k); s != nil {
					return s, nil
				}
			}
			pkg, err := p.checked(ctx, h, k)
			if err != nil {
				return nil, err
			}
			p.saveSummary(h, k, pkg.summary)
			return pkg.summary, nil
		})
		if err != nil || !k.renew(s) {
			return s, err
		}
		p.forget(k.key, s)
		fromDisk = false // the one on disk is stale too: it is written again
	}
}

// renew reports whether s, the summary of a check with the key k, is stale
// and a new check of k would not be: the summaries of the imports that k
// has are not stale. (Otherwise a new check is no better: a new snapshot
// makes a new key with new summaries, see Package.Stale.)
func (k *keyed) renew(s *summary) bool {
	if !s.stale() {
		return false
	}
	for _, d := range k.deps {
		if d != nil && d.stale() {
			return false
		}
	}
	return true
}

// forget drops the stale summary s of the key, and its checked package,
// from the caches.
func (p *Pipeline) forget(key Hash, s *summary) {
	p.summaries.ForgetIf(key, func(v *summary, _ error) bool { return v == s })
	p.checks.ForgetIf(key, func(v *Package, _ error) bool { return v != nil && v.summary == s })
}

// packageOf returns the checked package of h: from the cache, or checked
// now.
func (p *Pipeline) packageOf(ctx context.Context, h *handle) (*Package, error) {
	if h.cyclic {
		return h.own.Get(ctx)
	}
	if pkg := h.last.Load(); pkg != nil && !pkg.Stale() {
		return pkg, nil
	}
	k, err := h.keyed.Get(ctx)
	if err != nil {
		return nil, err
	}
	pkg, err := p.checked(ctx, h, k)
	if err == nil && h.open {
		h.last.Store(pkg)
	}
	return pkg, err
}

// checked returns the check of the key k of h: a new one when the kept one
// is stale and a new one would not be (keyed.renew).
func (p *Pipeline) checked(ctx context.Context, h *handle, k *keyed) (*Package, error) {
	for {
		pkg, err := p.checks.Get(ctx, k.key, func(ctx context.Context) (*Package, error) { return p.check(ctx, h, k) })
		if err != nil || !k.renew(pkg.summary) {
			return pkg, err
		}
		p.forget(k.key, pkg.summary)
	}
}

// check loads the package of h alone, with its imports from their
// summaries (k.deps), and type-checks it with their types from their
// export data, at most GOMAXPROCS checks at a time.
func (p *Pipeline) check(ctx context.Context, h *handle, k *keyed) (*Package, error) {
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-p.sem }()
	p.stats.checks.Add(1)

	pattern, dir := h.loadPattern()
	cfg := p.loadConfig(h, dir, false)
	cfg.Fset = token.NewFileSet()
	// The check reads what the resolution of its imports read, which its
	// key has (or, for a cyclic package, the snapshot that made h).
	cfg.FS = newReplayFS(h.meta.reads)
	if h.cyclic {
		cfg.FS = h.fs
	}
	// The imports from their export data, all into one map, so that a
	// package that two of them mention is one *types.Package.
	imported := make(map[string]*types.Package)
	depOf := make(map[*load.Package]check.Dep, len(k.deps))
	byRef := make(map[depRef]*summary, len(k.deps))
	for i, s := range k.deps {
		if s == nil {
			continue // cyclic: loaded from source
		}
		dep := h.deps[i]
		byRef[depRef{s.importPath, dep.id.dir, dep.id.std}] = s
		d := check.Dep{HasC: s.hasC, Failed: s.failed}
		if s.export != nil {
			pkg, err := gcexportdata.Read(bytes.NewReader(s.export), cfg.Fset, imported, s.importPath)
			if err != nil {
				return nil, fmt.Errorf("cache: the export data of %s: %v", s.importPath, err)
			}
			d.Types = pkg
		}
		depOf[s.load] = d
	}
	cfg.Deps = func(importPath, dir string, std bool) *load.Package {
		if s := byRef[depRef{importPath, dir, std}]; s != nil {
			return s.load
		}
		if h.cyclic {
			return nil // the loader loads it, and finds the cycle
		}
		// Not in the metadata: the metadata and the check resolved the
		// imports differently, which they can't (they read the same).
		// The import fails at its spec, never loaded from source behind
		// the key's back.
		errs := new(diag.List)
		errs.Errorf(token.Position{}, "internal error: the import of %s was not resolved with the metadata of %s", importPath, dir)
		return (&load.Package{ImportPath: importPath, Dir: dir, Std: std, Errors: errs, Incomplete: true}).Summary()
	}
	prog := load.Load(cfg, pattern)
	if len(prog.Roots) == 0 {
		return nil, fmt.Errorf("cache: no package in %s", dir)
	}
	root := prog.Roots[0]
	reportImportErrors(prog, root)
	sum := &summary{importPath: root.ImportPath, load: root.Summary()}

	// The compiler, the probes and pkg-config answer at once, from what
	// the prober knows (probe.go); rec keeps what they answered.
	rec := &probeRecord{pr: p.prober}
	r := check.Check(prog, check.Config{
		Phase:     p.cfg.Phase,
		GOOS:      p.cfg.GOOS,
		GOARCH:    p.cfg.GOARCH,
		Context:   ctx,
		CXX:       p.prober.findCXX(rec),
		Probe:     p.prober.probeFor(rec),
		PkgConfig: p.prober.pkgConfigFor(rec),
		Deps:      func(lp *load.Package) check.Dep { return depOf[lp] },
		// A package with load errors (a file that doesn't parse) is
		// type-checked too, for hover and go to definition there
		// (check.Package.Partial); its diagnostics stay the same.
		Partial: true,
	})
	if err := ctx.Err(); err != nil {
		return nil, err // the probe stopped
	}
	cp := r.Package(root)
	dep, _ := r.Dep(root)
	sum.hasC, sum.failed = dep.HasC, dep.Failed
	if dep.Types != nil {
		var buf bytes.Buffer
		if err := gcexportdata.Write(&buf, cfg.Fset, dep.Types); err != nil {
			return nil, fmt.Errorf("cache: the export data of %s: %v", root.ImportPath, err)
		}
		sum.export = buf.Bytes()
	}
	src := make(map[string][]byte, len(h.files))
	for _, f := range h.files {
		src[f.path] = f.src
	}
	if cp != nil && cp.Load == root {
		sum.opaque = opaqueTableOf(cfg.Fset, cp, func(name string) []byte { return src[name] })
	}
	sum.apiHash = apiHash(root, dep, k.deps, sum.opaque)
	if rec.used() {
		sum.records = append(sum.records, rec)
		sum.own = rec
	}
	sum.inherit(k.deps)
	pkg := &Package{Program: prog, Check: cp, APIHash: sum.apiHash, summary: sum,
		src: src, imported: imported, tables: sum.tables}
	for _, f := range h.files {
		pkg.cost += sourceFactor * int64(len(f.src))
	}
	for _, s := range k.deps {
		if s != nil {
			pkg.cost += exportFactor * int64(len(s.export))
		}
	}
	return pkg, nil
}

// exportFactor is how much memory the types of an import take in a checked
// package, per byte of its export data (measured with
// TestCostFactors: about 14).
const exportFactor = 14

// reportImportErrors reports, at each spec of root that imports a package
// summary, every error of that package that has no place (packageErrors).
// Those are the errors that a package's own load gives the package as a
// whole, which it reports at the import that loaded it, and so at no place
// when the package is a root, as each package of the pipeline is: no .gp
// files, no non-test .gp files, build constraints exclude all .gp files,
// found packages a and b, a folder that can't be read, and the internal
// error of an import that the metadata didn't resolve (check). A load of
// the whole program reports them once, at the first import; the pipeline
// reports them at every importing spec of every importer (a known
// difference, like gopls's "could not import"). An import cycle error has
// no place either, but it never gets here: a package in or above a cycle
// is cyclic, and its check loads the cyclic imports from source, so the
// loader reports the cycle itself.
func reportImportErrors(prog *load.Program, root *load.Package) {
	for _, f := range root.Files {
		if f.Info == nil {
			continue
		}
		for _, spec := range f.Info.Packages {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			dep := root.Imports[path]
			if dep == nil {
				continue
			}
			for _, d := range packageErrors(dep.Errors) {
				d.Pos = prog.Fset.Position(spec.Pos())
				root.Errors.Add(d)
			}
		}
	}
}

// packageErrors returns the errors of errs that have no place: those about
// the package as a whole (see reportImportErrors).
func packageErrors(errs *diag.List) []diag.Diagnostic {
	var list []diag.Diagnostic
	for _, d := range errs.Diagnostics() {
		if d.Pos.Filename == "" && d.Pos.Line == 0 && d.Severity == diag.Error {
			list = append(list, d)
		}
	}
	return list
}

// sourceFactor is how much memory a checked package takes per byte of its
// source: its ASTs, types and types.Info (measured with TestCostFactors).
const sourceFactor = 45
