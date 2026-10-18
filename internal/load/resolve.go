package load

import (
	"bytes"
	"fmt"
	"go/build"
	"go/token"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// CgohPath is the import path of the hidden package that the type checker
// gives the conversions of opaque values (package check: T(x) becomes
// gp_cgoh.As[T](x)). The package is made in Go code; no .gp file can import
// it, and a user package with this path (in a module named gocpp) can't be
// imported either: resolve reports it like an internal package.
const CgohPath = "gocpp/cgoh"

// A location is where the package with an import path is.
type location struct {
	importPath string
	dir        string // the absolute folder, or the folder in Config.StdlibDir
	std        bool
	mod        *Module
}

// resolve returns the package for the import of path in the package from,
// at the position at, or the error message if there is no such package.
// It returns nil and no error for "unsafe", which go/types knows itself.
// The package that it returns may not be loaded yet.
func (l *loader) resolve(from *Package, path string, at token.Position) (*Package, string) {
	if path == "unsafe" {
		return nil, ""
	}
	if path == CgohPath {
		return nil, fmt.Sprintf("use of internal package %s not allowed", path)
	}
	loc, msg := l.find(from, path)
	// Go's rule for internal packages comes first for a package that
	// exists, also for a standard package that gocpp doesn't have yet;
	// a path that names no package gets the error of find.
	if !internalAllowed(from, path) && (msg == "" || l.cfg.GoStd != nil && l.cfg.GoStd(path)) {
		return nil, fmt.Sprintf("use of internal package %s not allowed", path)
	}
	if msg != "" {
		return nil, msg
	}
	if loc.mod != nil && !loc.std && !from.Std && loc.mod != from.Module {
		// A package of another module: its cxx recipe (if any) builds the
		// C and C++ files the package needs (cxxrecipe.go).
		l.noteCxxModule(from, loc.mod)
	}
	if g := l.graph; g != nil {
		// In the graph of a test, the test variant has the path of the
		// package under test (so the external test package gets it too),
		// and a package that imports that path is loaded again.
		if p := g.byPath[loc.importPath]; p != nil {
			return p, ""
		}
		if p := l.byPath[loc.importPath]; p != nil && !g.reachesRoot(p) {
			return p, ""
		}
		p := l.newPackage(loc.importPath, loc.dir, loc.std, loc.mod)
		p.at, p.main = at, from.mainModule()
		g.byPath[loc.importPath] = p
		return p, ""
	}
	if p := l.byPath[loc.importPath]; p != nil {
		return p, ""
	}
	if l.cfg.Deps != nil && !l.cfg.Tests {
		if p := l.cfg.Deps(loc.importPath, loc.dir, loc.std); p != nil {
			if !p.summary {
				panic("load: Config.Deps returned a package that Package.Summary did not make")
			}
			l.byPath[loc.importPath] = p
			l.state[p] = loaded
			return p, ""
		}
	}
	p := l.newPackage(loc.importPath, loc.dir, loc.std, loc.mod)
	p.at, p.main = at, from.mainModule()
	l.byPath[loc.importPath] = p
	return p, ""
}

// internalAllowed reports whether the package from may import the package
// with the import path path, by Go's rule for internal packages: a path
// with an element internal can be imported only by the packages in the
// tree of the folder above its last internal element. For
// "example.com/app/a/internal/x", that is example.com/app/a and the
// packages below it; "internal/bytealg" is only for standard packages.
// The .gp files named on the command line count as the package of their
// folder, and an external test package as its package.
func internalAllowed(from *Package, path string) bool {
	var parent string
	switch {
	case strings.HasSuffix(path, "/internal"):
		parent = strings.TrimSuffix(path, "/internal")
	case strings.Contains(path, "/internal/"):
		parent = path[:strings.LastIndex(path, "/internal/")]
	case path == "internal" || strings.HasPrefix(path, "internal/"):
		return from.Std
	default:
		return true
	}
	importer := from.ImportPath
	if from.xtestOf != nil {
		importer = from.xtestOf.ImportPath
	}
	if mod := from.Module; importer == commandLine && mod != nil && mod.Path != "" {
		if rel, err := filepath.Rel(mod.Dir, from.Dir); err == nil {
			importer = joinImportPath(mod.Path, filepath.ToSlash(rel))
		}
	}
	return inModule(importer, parent)
}

// find returns where the package with the import path path is, for an
// import in the package from, or the error message if there is no such
// package. For a pattern that is an import path, from is a package with
// no import path, in Config.Dir. The messages are those of the go
// command, with gocpp's file names.
func (l *loader) find(from *Package, path string) (location, string) {
	if msg := checkImportPath(path); msg != "" {
		return location{}, msg
	}
	if l.isStd(path) {
		return location{importPath: path, dir: l.stdDir(path), std: true}, ""
	}
	if from.Std {
		// A standard package can import only standard packages.
		return location{}, l.notInStd(path)
	}
	own := -1 // the length of the path of the module of from, if path is in it
	if mod := from.Module; mod != nil && mod.Path != "" && inModule(path, mod.Path) {
		own = len(mod.Path)
	}
	// A required module whose path is longer than the module's own is the
	// one that has the package (Go: the longest module path wins).
	if req := l.findRequire(from, path); req != nil && len(req.Path) > own {
		return l.inRequired(from, req, path)
	}
	if own >= 0 {
		mod := from.Module
		dir := filepath.Join(mod.Dir, filepath.FromSlash(strings.TrimPrefix(path[len(mod.Path):], "/")))
		if !l.isDir(dir) {
			return location{}, fmt.Sprintf("package %s is not in module %s (%s)", path, mod.Path, dir)
		}
		if !mod.Vendored && l.inOtherModule(mod, dir) {
			// A gocpp.mod file between dir and the module root: dir is in
			// another module, which the main module must require. The
			// messages are go build's: for a pattern (from is no package
			// then), and for an import.
			if from.ImportPath == "" {
				return location{}, fmt.Sprintf("main module (%s) does not contain package %s", mod.Path, path)
			}
			return location{}, "no required module provides package " + path
		}
		return location{importPath: path, dir: dir, mod: mod}, ""
	}
	first, _, _ := strings.Cut(path, "/")
	switch {
	case l.cfg.GoStd != nil && l.cfg.GoStd(path) || !strings.Contains(first, "."):
		return location{}, l.notInStd(path)
	case from.Module == nil:
		return location{}, fmt.Sprintf("no required module provides package %s: "+
			"gocpp.mod file not found in %s or any parent directory", path, from.Dir)
	}
	return location{}, "no required module provides package " + path + l.replacedHint(from, path)
}

// notInStd returns the error for the import of a path that is not a
// standard package that gocpp has: a package of Go's standard library
// comes in Phase 09.
func (l *loader) notInStd(path string) string {
	if l.cfg.GoStd != nil && l.cfg.GoStd(path) {
		return fmt.Sprintf("package %s is not available yet (Phase 09)", path)
	}
	return fmt.Sprintf("package %s is not in std (%s)", path, l.stdDir(path))
}

// isStd reports whether Config.Stdlib has a folder for the import path
// with a .gp file in it.
func (l *loader) isStd(path string) bool {
	if l.cfg.Stdlib == nil || !fs.ValidPath(path) {
		return false
	}
	if ok, seen := l.stdDirs[path]; seen {
		return ok
	}
	ok := false
	entries, _ := fs.ReadDir(l.cfg.Stdlib, path)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".gp") {
			ok = true
			break
		}
	}
	l.stdDirs[path] = ok
	return ok
}

// stdDir returns the folder of the standard package path, in
// Config.StdlibDir.
func (l *loader) stdDir(path string) string {
	return filepath.Join(l.cfg.StdlibDir, filepath.FromSlash(path))
}

// inModule reports whether the import path is in the module with the
// path mod.
func inModule(path, mod string) bool {
	return path == mod || strings.HasPrefix(path, mod+"/")
}

// checkImportPath returns the error for an import path that can't name a
// package, with the messages of the go command, or "". ClassifyImports
// reports the paths that start with ./ or ../ already (only a header can),
// and go/types the paths with characters that a path can't have.
func checkImportPath(path string) string {
	switch {
	case path == "." || path == ".." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../"):
		return fmt.Sprintf("%q is relative, but relative import paths are not supported", path)
	case strings.HasPrefix(path, "/"):
		return fmt.Sprintf("%q is not a package path", path)
	case strings.Contains(path, "//"):
		return fmt.Sprintf("malformed import path %q: double slash", path)
	case strings.HasSuffix(path, "/"):
		return fmt.Sprintf("malformed import path %q: trailing slash", path)
	}
	for elem := range strings.SplitSeq(path, "/") {
		if elem == "." || elem == ".." {
			return fmt.Sprintf("malformed import path %q: invalid path element %q", path, elem)
		}
	}
	// The characters of module.CheckImportPath: ASCII letters and digits and -._~+ (the generated
	// files are in a folder of this name, and the namespace is made of it).
	for _, r := range path {
		if r >= utf8.RuneSelf || !(r == '-' || r == '.' || r == '_' || r == '~' || r == '+' || r == '/' ||
			'0' <= r && r <= '9' || 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z') {
			return fmt.Sprintf("malformed import path %q: invalid char %q", path, r)
		}
	}
	return ""
}

// modFile is the name of the file at the top of a module.
const modFile = "gocpp.mod"

// FindModule returns the module of the absolute folder dir, as Load finds
// it: the one whose gocpp.mod file is in dir or in the nearest folder above
// it, or nil if there is none. Its Path is "" if gocpp.mod has no valid
// module line (Load reports why). gocpp check uses it to match patterns
// with "..." the way the go command does.
func FindModule(dir string) *Module {
	return newLoader(Config{Dir: dir}).module(filepath.Clean(dir))
}

// module returns the module of the folder dir: the one with the gocpp.mod
// file in dir or in the nearest folder above it, or nil if there is none.
// It reads each gocpp.mod file once, and reports its errors in
// Program.Errors.
func (l *loader) module(dir string) *Module {
	var walked []string
	var mod *Module
	for d := dir; ; {
		if m, ok := l.modules[d]; ok {
			mod = m
			break
		}
		walked = append(walked, d)
		name := filepath.Join(d, modFile)
		if data, err := l.readAbs(name); err == nil {
			mod = l.readModFile(name, d, data)
			break
		} else if fi, statErr := l.cfg.FS.Stat(name); statErr == nil && !fi.IsDir() {
			l.prog.Errors.Errorf(token.Position{Filename: name}, "%s", err)
			mod = &Module{Dir: d}
			break
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	for _, d := range walked {
		l.modules[d] = mod
	}
	return mod
}

// readModFile reads the gocpp.mod file name with the contents data, and
// reports its errors in Program.Errors, like the go command (file:line:
// message). It returns the module of the file (its Path is "" after an
// error in the module line).
func (l *loader) readModFile(name, dir string, data []byte) *Module {
	f, errs := gocppmod.Parse(name, data, func(p string) (bool, string) {
		return checkImportPath(p) == "" && goImportPathOK(p) && !IsHeaderOrNamespace(p), ""
	})
	for _, e := range errs {
		pos := token.Position{Filename: e.Filename, Line: e.Pos.Line}
		if e.Pos.LineRune > 1 {
			pos.Column = e.Pos.LineRune
		}
		l.prog.Errors.Add(diag.Diagnostic{Pos: pos, Msg: e.Err})
	}
	if !f.HasModuleLine() && len(data) <= gocppmod.MaxFileSize {
		l.prog.Errors.Errorf(token.Position{Filename: name}, "missing module declaration")
	}
	mod := &Module{Dir: dir, file: f}
	if f.Module != nil {
		mod.Path = f.Module.Path
	}
	return mod
}

// A matcher decides which .gp files of a folder are in the package, by
// their names and their build constraints, with the rules of go/build:
// x_linux.gp and //go:build linux are only for Linux, x_amd64.gp only for
// amd64, and the tags are GOOS, GOARCH, unix (on Unix systems), gocpp (the
// compiler, as gc is for Go's), the release tags go1.1 to go1.25, and
// Config.Tags. cgo is not satisfied: gocpp has no cgo (import "C" is an
// error), so the //go:build !cgo files of Go's standard library are the
// ones that are ported.
type matcher struct {
	ctxt build.Context
	src  []byte // the file that ctxt.OpenFile gives
}

// goMinor is the minor version of the Go language that gocpp pins
// (types.Config.GoVersion, Phase 01 README §2.7): the release tags are
// go1.1 to go1.<goMinor>.
const goMinor = 25

func newMatcher(goos, goarch string, tags []string) *matcher {
	m := &matcher{}
	m.ctxt = build.Context{
		GOOS:      goos,
		GOARCH:    goarch,
		Compiler:  "gocpp",
		BuildTags: tags,
		OpenFile: func(string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(m.src)), nil
		},
	}
	for i := 1; i <= goMinor; i++ {
		m.ctxt.ReleaseTags = append(m.ctxt.ReleaseTags, fmt.Sprintf("go1.%d", i))
	}
	return m
}

// file reports whether the .gp file name with the contents src is in the
// package. It returns the message for a broken build constraint (then the
// file is not in the package), such as "multiple //go:build comments".
//
// It uses go/build's MatchFile, which knows .go files and not .gp files, so
// it asks about the same name with .go. A file whose package clause or
// imports MatchFile can't read is in the package: the parser reports the
// error. Other files (a C source: x_linux.c, //go:build at its top) keep
// their names.
func (m *matcher) file(name string, src []byte) (bool, string) {
	goName := name
	if strings.HasSuffix(name, ".gp") {
		goName = strings.TrimSuffix(name, ".gp") + ".go"
	}
	m.src = src
	ok, err := m.ctxt.MatchFile("", goName)
	m.src = nil
	switch {
	case err == nil:
		return ok, ""
	case ok:
		return true, "" // MatchFile couldn't read the header
	}
	return false, strings.TrimPrefix(err.Error(), goName+": ")
}
