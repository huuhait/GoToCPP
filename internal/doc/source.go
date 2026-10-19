package doc

import (
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"path"
	"runtime"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// A File is the source of one .gp file of a package.
type File struct {
	Name string // the base name: "list.gp"
	Src  []byte
}

// A Package is the source of one gocpp package: what Render documents. It
// holds text, not syntax trees, because rendering changes the trees (like
// go doc, which runs once); each Render parses the files again, which is
// cheap, so a Package can be rendered any number of times, also at the same
// time.
type Package struct {
	// ImportPath is the import path ("encoding/json", "example.com/app/geom"),
	// or "." for a folder that is in no module.
	ImportPath string

	// Name is the name from the package clause.
	Name string

	// Dir is the folder of the package for messages and for the positions
	// of its files. For a standard package it is in Env.StdlibDir.
	Dir string

	// Std reports whether the package is a standard package.
	Std bool

	// Files are the .gp files of the package that the build constraints
	// keep, without the test files, sorted by name.
	Files []File

	// TestFiles are the _test.gp files of the package (with its own package
	// clause), sorted by name. Only the examples (Options.Examples) read
	// them.
	TestFiles []File
}

// parseMode is how the files are read: with comments, with the object
// resolution (go doc needs Obj of the identifier error), and with the syntax
// of Phase 10.
const parseMode = parser.ParseComments | parser.Phase10Syntax

// A parsed is a Package after parsing, with what Render needs to know about
// the syntax of Phase 10.
type parsed struct {
	fset      *token.FileSet
	files     []*ast.File
	testFiles []*ast.File

	// macros are the macro declarations. go/doc sorts and filters by name,
	// and it takes "#Assert" for an unexported name, so while go/doc runs
	// they are named "Assert#" (see mangle); after it, each name is "#Assert"
	// again, as the printer wants it.
	macros []*ast.FuncDecl

	// onTop holds the lines of macros that are on top of a func or a type
	// declaration (gpast.OnTop), by that declaration, as source text.
	onTop map[ast.Node][]string

	// onTopCalls are the same lines as syntax, for #Derive.
	onTopCalls map[ast.Node][]*ast.CallExpr
}

// parse reads the files of the package.
func (p *Package) parse() (*parsed, error) {
	ps := &parsed{fset: token.NewFileSet(), onTop: make(map[ast.Node][]string), onTopCalls: make(map[ast.Node][]*ast.CallExpr)}
	read := func(files []File) ([]*ast.File, error) {
		var out []*ast.File
		for _, f := range files {
			af, err := parser.ParseFile(ps.fset, path.Join(p.Dir, f.Name), f.Src, parseMode)
			if err != nil {
				return nil, err
			}
			out = append(out, af)
		}
		return out, nil
	}
	var err error
	if ps.files, err = read(p.Files); err != nil {
		return nil, err
	}
	if len(ps.files) == 0 {
		return nil, fmt.Errorf("no source-code package in directory %s", p.Dir)
	}
	if ps.testFiles, err = read(p.TestFiles); err != nil {
		return nil, err
	}
	for _, f := range ps.files {
		ps.scanPhase10(f)
	}
	return ps, nil
}

// scanPhase10 notes the macros of the file f and renames them for go/doc
// (mangle).
func (ps *parsed) scanPhase10(f *ast.File) {
	for i, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if gpast.IsMacroDecl(d) {
				ps.macros = append(ps.macros, d)
				d.Name.Name = mangle(d.Name.Name)
			}
			ps.noteOnTop(f, i, d)
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				ps.noteOnTop(f, i, d)
			}
		}
	}
}

// noteOnTop records the lines of macros on top of the declaration f.Decls[i].
func (ps *parsed) noteOnTop(f *ast.File, i int, d ast.Decl) {
	var lines []string
	calls := gpast.OnTopOf(f.Decls, i)
	for _, call := range calls {
		var sb strings.Builder
		if err := formatNode(&sb, ps.fset, call); err == nil {
			lines = append(lines, sb.String())
		}
	}
	if lines != nil {
		ps.onTop[d] = lines
		ps.onTopCalls[d] = calls
	}
}

// mangle returns the name of the macro "#Name" as go/doc sees it: "Name#",
// which is exported when the macro is, and sorts right after a func Name.
func mangle(name string) string { return gpast.MacroBase(name) + "#" }

// restoreMacros gives the macros their names, after go/doc is done.
func (ps *parsed) restoreMacros() {
	for _, d := range ps.macros {
		d.Name.Name = "#" + strings.TrimSuffix(d.Name.Name, "#")
	}
}

// A nameKind says what a program name is, for the matching of the
// symbols the user types.
type nameKind int

const (
	plainName nameKind = iota // Name
	mustName                  // Name!
	macroName                 // Name#, which is a macro (#Name) for go/doc
)

// splitName returns the name without its mark, and the mark.
func splitName(name string) (string, nameKind) {
	switch {
	case strings.HasSuffix(name, "#"):
		return strings.TrimSuffix(name, "#"), macroName
	case strings.HasSuffix(name, "!") && len(name) > 1:
		return strings.TrimSuffix(name, "!"), mustName
	}
	return name, plainName
}

// goTarget returns the GOOS and GOARCH of the environment.
func (e *Env) goTarget() (goos, goarch string) {
	goos, goarch = e.GOOS, e.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

// readPackage reads the package in the folder rel of fsys (whose name in
// messages is dir), with the import path importPath. It is
// build.ImportDir: a folder with no .gp file that the constraints keep, or
// with two package names, is an error.
func (e *Env) readPackage(fsys fs.FS, rel, dir, importPath string, std bool) (*Package, error) {
	entries, err := fs.ReadDir(fsys, rel)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %v", dir, errorText(err))
	}
	goos, goarch := e.goTarget()
	pkg := &Package{ImportPath: importPath, Dir: dir, Std: std}
	var firstFile string
	var testNames, haveTests = map[string]string{}, false
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".gp") || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		src, err := fs.ReadFile(fsys, path.Join(rel, name))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %v", path.Join(dir, name), errorText(err))
		}
		ok, msg := load.MatchFile(goos, goarch, e.Tags, name, src)
		if msg != "" {
			return nil, fmt.Errorf("%s: %s", path.Join(dir, name), msg)
		}
		if !ok {
			continue
		}
		// The package clause only: the whole file is parsed when it is rendered.
		f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.PackageClauseOnly|parser.Phase10Syntax)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", path.Join(dir, name), errorText(err))
		}
		pn := f.Name.Name
		if strings.HasSuffix(name, "_test.gp") {
			haveTests = true
			testNames[name] = pn
			pkg.TestFiles = append(pkg.TestFiles, File{name, src})
			continue
		}
		if pn == "documentation" { // like go/build: files that only document
			continue
		}
		if pkg.Name == "" {
			pkg.Name, firstFile = pn, name
		} else if pn != pkg.Name {
			return nil, fmt.Errorf("found packages %s (%s) and %s (%s) in %s", pkg.Name, firstFile, pn, name, dir)
		}
		pkg.Files = append(pkg.Files, File{name, src})
	}
	if len(pkg.Files) == 0 {
		if haveTests {
			return nil, fmt.Errorf("no source-code package in directory %s%s", dir, onlyTestFilesNote)
		}
		return nil, fmt.Errorf("no buildable .gp source files in %s", dir)
	}
	// Keep the test files of the package itself and of its external test
	// package p_test (Go's usual place for examples).
	pkg.TestFiles = slices.DeleteFunc(pkg.TestFiles, func(f File) bool {
		return testNames[f.Name] != pkg.Name && testNames[f.Name] != pkg.Name+"_test"
	})
	return pkg, nil
}

// onlyTestFilesNote ends the error for a folder with only test files.
const onlyTestFilesNote = " (only _test.gp files)"

// errorText returns the text of an error without the "open /x: " that a path
// error starts with, which the callers add themselves.
func errorText(err error) string {
	if pe, ok := err.(*fs.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
