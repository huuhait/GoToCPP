package cache

import (
	"bytes"
	"context"
	"fmt"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/go/types/objectpath"

	"github.com/huuhait/gocpp/internal/lsp/memoize"
)

// The API of packages that a document does not import yet, for the
// completion of their members with an import to add (step 11.6): the
// language server asks for the package of a folder or a standard package
// by its import path, and gets its types from its summary, as an importer
// would.

// An Exports is the API of a package as its importers see it: its types
// from its export data, in a FileSet of their own (the positions have the
// file and the line only, see declOf in the server), and the spellings of
// its C/C++ types (OpaqueType). It is shared, and must not be changed.
type Exports struct {
	Types *types.Package
	Fset  *token.FileSet

	opaque map[types.Object]string
}

// OpaqueType is Package.OpaqueType for the objects of e.Types.
func (e *Exports) OpaqueType(obj types.Object) (string, bool) {
	switch o := obj.(type) {
	case nil:
		return "", false
	case *types.Var:
		obj = o.Origin()
	case *types.Func:
		obj = o.Origin()
	}
	t, ok := e.opaque[obj]
	return t, ok
}

// Exports returns the API of the package in the folder dir (an absolute
// name), or, with std, of the standard package importPath: from its summary
// (in memory, from the disk cache, or after a check of the package), decoded
// once per summary. It is nil when the package has no types (it could not be
// type-checked).
func (s *Snapshot) Exports(ctx context.Context, dir string, std bool, importPath string) (*Exports, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	id := pkgID{dir: filepath.Clean(dir)}
	if std {
		id = pkgID{dir: filepath.Join(st.p.cfg.StdlibDir, filepath.FromSlash(importPath)), std: true}
	} else {
		importPath = ""
	}
	h, err := st.handle(ctx, id, importPath, nil)
	if err != nil {
		return nil, err
	}
	sum, err := st.p.summaryOf(ctx, h)
	if err != nil {
		return nil, err
	}
	sum.expOnce.Do(func() { sum.exp, sum.expErr = sum.decode() })
	return sum.exp, sum.expErr
}

// decode decodes the export data of s and finds the objects of its side
// table of opaque types.
func (s *summary) decode() (*Exports, error) {
	if s.export == nil {
		return nil, nil
	}
	fset := token.NewFileSet()
	pkg, err := gcexportdata.Read(bytes.NewReader(s.export), fset, make(map[string]*types.Package), s.importPath)
	if err != nil {
		return nil, fmt.Errorf("cache: the export data of %s: %v", s.importPath, err)
	}
	e := &Exports{Types: pkg, Fset: fset}
	for _, entry := range s.opaque {
		obj, err := objectpath.Object(pkg, entry.Path)
		if err != nil {
			continue
		}
		if e.opaque == nil {
			e.opaque = make(map[types.Object]string)
		}
		e.opaque[obj] = entry.Type
	}
	return e, nil
}

// StdPackages returns the import paths of the standard packages, sorted:
// the folders of Config.Stdlib that have .gp files. It reads the folders
// once.
func (p *Pipeline) StdPackages() []string {
	p.stdOnce.Do(func() {
		if p.cfg.Stdlib == nil {
			return
		}
		fs.WalkDir(p.cfg.Stdlib, ".", func(name string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(name, ".gp") {
				if dir := path.Dir(name); dir != "." && !slices.Contains(p.stdPaths, dir) {
					p.stdPaths = append(p.stdPaths, dir)
				}
			}
			return nil
		})
		slices.Sort(p.stdPaths)
	})
	return p.stdPaths
}

// StdFiles returns the absolute names (in Config.StdlibDir) and the content
// of the .gp files of the standard package importPath, sorted by name. The
// content must not be changed.
func (p *Pipeline) StdFiles(importPath string) (names []string, srcs [][]byte) {
	for _, f := range p.stdFiles(importPath) {
		if f.err == nil {
			names = append(names, f.path)
			srcs = append(srcs, f.src)
		}
	}
	return names, srcs
}

// The list of the standard packages (StdPackages).
type stdList struct {
	stdOnce  sync.Once
	stdPaths []string
}

// PackageWithFile returns the checked package of the folder of the file
// path (an absolute name), as Snapshot.Package does, but with src as the
// content of that file. It is for the features that check a fixed text
// (completion after x. at the end of a line, see the server's
// fixDangling): the check is cached by its key, which has the hash of src,
// so it is never the check of the snapshot's own text, which the
// diagnostics use. The file must be a file of its package in the snapshot.
// The check store keeps only the latest keptVersions patched checks of each package, separately from
// ordinary versions and still under its global cost cap. Eviction drops cache ownership only; callers
// may keep using a returned package for as long as they hold it.
func (s *Snapshot) PackageWithFile(ctx context.Context, path string, src []byte) (*Package, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	h, err := st.handle(ctx, pkgID{dir: filepath.Dir(filepath.Clean(path))}, "", nil)
	if err != nil {
		return nil, err
	}
	v := &handle{id: h.id, importPath: h.importPath, mod: h.mod, meta: h.meta, deps: h.deps, cut: h.cut,
		cyclic: h.cyclic, fs: h.fs, variant: true}
	found := false
	for _, f := range h.files {
		if f.path == path {
			f = &fileHandle{path: path, src: src, hash: hashOf(src)}
			found = true
		}
		v.files = append(v.files, f)
	}
	if !found {
		return nil, fmt.Errorf("cache: %s is no file of its package", path)
	}
	// The imports are the same: the header of the file must be (the
	// metadata of h is reused).
	x := newHasher("variant")
	x.hash(h.localKey)
	x.str(path)
	x.hash(hashOf(src))
	v.localKey = x.sum()
	p := st.p
	v.keyed = memoize.NewPromise(func(ctx context.Context) (*keyed, error) { return p.keyOf(ctx, v) })
	if v.cyclic {
		v.own = memoize.NewPromise(func(ctx context.Context) (*Package, error) {
			k, err := v.keyed.Get(ctx)
			if err != nil {
				return nil, err
			}
			return p.check(ctx, v, k, nil)
		})
	}
	return p.packageOf(ctx, v)
}
