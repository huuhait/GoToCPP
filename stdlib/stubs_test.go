package stdlib_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/stdlib"
)

// stubPackages returns the import paths of the packages in the embedded
// FS: the folders that hold .gp files.
func stubPackages(t *testing.T) []string {
	t.Helper()
	var paths []string
	err := fs.WalkDir(stdlib.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".gp" {
			return err
		}
		if dir := path.Dir(p); !slices.Contains(paths, dir) {
			paths = append(paths, dir)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// TestStubsParse checks that each .gp file in the FS parses with the
// parser fork without errors, is formatted like gocpp fmt formats it, has
// the package name of its folder, and is in the folder of a standard
// package.
func TestStubsParse(t *testing.T) {
	err := fs.WalkDir(stdlib.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".gp" {
			return err
		}
		src, err := fs.ReadFile(stdlib.FS(), p)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, src, parser.ParseComments|parser.AllErrors)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			return nil
		}
		if formatted, err := format.Source(p, src); err != nil || !bytes.Equal(formatted, src) {
			t.Errorf("%s is not formatted (gocpp fmt -l), error %v", p, err)
		}
		dir := path.Dir(p)
		if f.Name.Name != path.Base(dir) {
			t.Errorf("%s: package %s, want %s", p, f.Name.Name, path.Base(dir))
		}
		if !stdlib.IsStd(dir) {
			t.Errorf("%s: %s is not a standard package", p, dir)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// loadAndCheck loads the patterns in the folder dir with load.Load and the
// embedded stubs, as the front-end does, and type-checks every loaded
// package in order with stock go/types and FakeImportC, as step 1.10 will.
// It returns the errors of both steps, with file names relative to dir.
func loadAndCheck(t *testing.T, cfg load.Config, dir string, patterns ...string) (*load.Program, string) {
	t.Helper()
	cfg.Dir, cfg.Stdlib, cfg.GoStd = dir, stdlib.FS(), stdlib.IsStd
	prog := load.Load(cfg, patterns...)
	var b bytes.Buffer
	pr := diag.Printer{All: true, Dir: dir}
	print := func(name string, l *diag.List) {
		if len(l.Diagnostics()) == 0 {
			return
		}
		if name != "" {
			fmt.Fprintf(&b, "# %s\n", name)
		}
		if err := pr.Print(&b, l); err != nil {
			t.Fatal(err)
		}
	}
	print("", prog.Errors)
	for _, p := range prog.Packages {
		print(p.ImportPath, p.Errors)
	}
	if prog.HasErrors() {
		return prog, b.String() // go/types runs only on packages that load
	}
	checked := make(map[*load.Package]*types.Package)
	for _, p := range prog.Packages {
		var errs diag.List
		conf := types.Config{
			GoVersion:   "go1.25",
			FakeImportC: true,
			Importer: importerFunc(func(path string) (*types.Package, error) {
				if path == "unsafe" {
					return types.Unsafe, nil // go/types has it built in
				}
				if tp := checked[p.Imports[path]]; tp != nil {
					return tp, nil
				}
				return nil, fmt.Errorf("package %s is not checked", path)
			}),
			Error: errs.Collect,
		}
		files := make([]*ast.File, len(p.Files))
		for i, f := range p.Files {
			files[i] = f.AST
		}
		checked[p], _ = conf.Check(p.ImportPath, prog.Fset, files, nil)
		print(p.ImportPath+" (go/types)", &errs)
	}
	return prog, b.String()
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// TestStubsTypeCheck loads every stub package with load.Load and
// type-checks it with stock go/types, with 0 errors.
func TestStubsTypeCheck(t *testing.T) {
	// (The packages of stdlib.WindowsOnly have no file for the default system, Linux.)
	paths := slices.DeleteFunc(stubPackages(t), func(p string) bool { return slices.Contains(stdlib.WindowsOnly, p) })
	prog, errs := loadAndCheck(t, load.Config{}, t.TempDir(), paths...)
	if errs != "" {
		t.Errorf("errors:\n%s", errs)
	}
	if n := len(prog.Roots); n != len(paths) {
		t.Errorf("%d packages loaded, want %d", n, len(paths))
	}
}

// laterSyntax are the examples that use the syntax of Phase 10, which the stock
// parser rejects and stock go/types can't check: internal/check rewrites it
// (its TestExamples checks these two, with no gate since step 10.12). Their
// packages have stubs all the same (encoding/json, sync, bufio, errors, ...).
var laterSyntax = map[string]string{
	"macros":      "macro declarations and #Name(...) calls (Phase 10)",
	"must-errors": "must functions (func f!()) and F!(...) calls (Phase 10)",
}

// TestExamplesTypeCheck loads each target program in ../examples with
// load.Load and the stubs, as gocpp check will, and type-checks it with
// stock go/types, with 0 errors: once as a program, and once with its
// _test.gp files (Config.Tests). The multi-pkg example's own packages come
// from its module (gocpp.mod). The examples in laterSyntax are skipped; the
// test checks that they still don't parse, so the list stays true.
func TestExamplesTypeCheck(t *testing.T) {
	dirs, err := filepath.Glob("../examples/*/main.gp")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 19 {
		t.Errorf("found %d examples, want 19", len(dirs))
	}
	for _, mainFile := range dirs {
		dir, err := filepath.Abs(filepath.Dir(mainFile))
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			if why, ok := laterSyntax[name]; ok {
				if _, err := parser.ParseFile(token.NewFileSet(), mainFile, nil, parser.ParseComments); err == nil {
					t.Errorf("%s parses now: take it out of laterSyntax", name)
				}
				t.Skip("uses " + why)
			}
			// An example for one system (win32) is checked for it; the
			// others are for every system, so they are checked for both.
			systems := 0
			for _, goos := range []string{"linux", "windows"} {
				for _, tests := range []bool{false, true} {
					prog, errs := loadAndCheck(t, load.Config{GOOS: goos, GOARCH: "amd64", Tests: tests}, dir, ".")
					if ds := prog.Roots[0].Errors.Diagnostics(); len(ds) == 1 &&
						strings.Contains(ds[0].Msg, "build constraints exclude all .gp files") {
						continue
					}
					systems++
					if errs != "" {
						t.Errorf("%s, Tests %v:\n%s", goos, tests, errs)
					}
				}
			}
			if systems == 0 {
				t.Errorf("the build constraints exclude every file on linux and windows")
			}
		})
	}
}
