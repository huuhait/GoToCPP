package stdlib_test

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"os"
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

// An importer type-checks the stubs from the embedded FS and, for the
// multi-pkg example, the packages of one module from its folder. It loads
// the imports of a package first, so the packages are checked in
// dependency order, and it keeps each package it has checked.
type importer struct {
	t       *testing.T
	fset    *token.FileSet
	modPath string // the module path of the example, or ""
	modDir  string // the module's folder
	pkgs    map[string]*types.Package
	loading map[string]bool
}

func newImporter(t *testing.T, modPath, modDir string) *importer {
	return &importer{t: t, fset: token.NewFileSet(), modPath: modPath, modDir: modDir,
		pkgs: make(map[string]*types.Package), loading: make(map[string]bool)}
}

func (imp *importer) Import(p string) (*types.Package, error) {
	if p == "unsafe" {
		return types.Unsafe, nil // go/types has it built in
	}
	if pkg := imp.pkgs[p]; pkg != nil {
		return pkg, nil
	}
	if imp.loading[p] {
		return nil, fmt.Errorf("import cycle not allowed: %s", p)
	}
	imp.loading[p] = true
	defer delete(imp.loading, p)

	var files []*ast.File
	var err error
	switch {
	case imp.modPath != "" && (p == imp.modPath || strings.HasPrefix(p, imp.modPath+"/")):
		dir := strings.TrimPrefix(strings.TrimPrefix(p, imp.modPath), "/")
		files, _, err = imp.parseDir(os.DirFS(imp.modDir), dir, imp.modDir, false)
	case stdlib.IsStd(p):
		files, _, err = imp.parseDir(stdlib.FS(), p, "", false)
	default:
		return nil, fmt.Errorf("%s is not a standard package", p)
	}
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("package %s has no stub", p)
	}
	pkg, errs := imp.check(p, files)
	for _, e := range errs {
		imp.t.Errorf("package %s: %s", p, e)
	}
	imp.pkgs[p] = pkg
	return pkg, nil
}

// parseDir parses the .gp files of the folder dir of fsys with the parser
// fork, and the _test.gp files too when tests is true. It returns the files
// and the number of _test.gp files among them. The file names in the
// FileSet are the paths in fsys, after the folder base when it isn't "".
func (imp *importer) parseDir(fsys fs.FS, dir, base string, tests bool) ([]*ast.File, int, error) {
	if dir == "" {
		dir = "."
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, 0, err
	}
	var files []*ast.File
	nTests := 0
	for _, e := range entries {
		name := e.Name()
		isTest := strings.HasSuffix(name, "_test.gp")
		if e.IsDir() || path.Ext(name) != ".gp" || (isTest && !tests) {
			continue
		}
		src, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, 0, err
		}
		filename := path.Join(dir, name)
		if base != "" {
			filename = filepath.Join(base, filepath.FromSlash(filename))
		}
		f, err := parser.ParseFile(imp.fset, filename, src, parser.ParseComments|parser.AllErrors)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, f)
		if isTest {
			nTests++
		}
	}
	return files, nTests, nil
}

// check prepares the files of the package p for go/types, as the loader
// will (load.ClassifyImports and load.RewriteFile: headers and namespaces
// become C.X), and type-checks them with stock go/types and FakeImportC.
// It returns the errors of both steps.
func (imp *importer) check(p string, files []*ast.File) (*types.Package, []string) {
	var list diag.List
	pkgLevel := load.PackageLevelNames(files)
	for _, f := range files {
		info := load.ClassifyImports(imp.fset, f, pkgLevel, nil, &list)
		load.RewriteFile(imp.fset, f, info, pkgLevel, nil, &list)
	}
	conf := types.Config{GoVersion: "go1.25", FakeImportC: true, Importer: imp, Error: list.Collect}
	pkg, _ := conf.Check(p, imp.fset, files, nil)
	var errs []string
	for _, d := range list.Diagnostics() {
		errs = append(errs, fmt.Sprintf("%s: %s", d.Pos, d.Msg))
	}
	return pkg, errs
}

// TestStubsTypeCheck type-checks every stub package with stock go/types,
// with 0 errors.
func TestStubsTypeCheck(t *testing.T) {
	imp := newImporter(t, "", "")
	for _, p := range stubPackages(t) {
		if _, err := imp.Import(p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// laterSyntax are the examples that use syntax that the parser takes only
// from Phase 10 on, so they can't be type-checked yet. Their packages have
// stubs all the same (encoding/json, sync, bufio, errors, ...).
var laterSyntax = map[string]string{
	"macros":      "macro declarations and #Name(...) calls (Phase 10)",
	"must-errors": "must functions (func f!()) and F!(...) calls (Phase 10)",
}

// TestExamplesTypeCheck type-checks each target program in ../examples
// against the stubs, with 0 errors: the package of main.gp, and again with
// its _test.gp files when it has some. The multi-pkg example's own
// packages are checked when main imports them. Files with header imports
// get the rewrites of step 1.8 first (FakeImportC then takes C.X). The
// examples in laterSyntax are skipped; the test checks that they still
// don't parse, so the list stays true.
func TestExamplesTypeCheck(t *testing.T) {
	dirs, err := filepath.Glob("../examples/*/main.gp")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 19 {
		t.Errorf("found %d examples, want 19", len(dirs))
	}
	for _, mainFile := range dirs {
		dir := filepath.Dir(mainFile)
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			modPath := modulePath(t, dir)
			imp := newImporter(t, modPath, dir)
			if why, ok := laterSyntax[name]; ok {
				if _, _, err := imp.parseDir(os.DirFS(dir), ".", dir, false); err == nil {
					t.Errorf("%s parses now: take it out of laterSyntax", name)
				}
				t.Skip("uses " + why)
			}
			for _, tests := range []bool{false, true} {
				files, nTests, err := imp.parseDir(os.DirFS(dir), ".", dir, tests)
				if err != nil {
					t.Fatal(err)
				}
				if tests && nTests == 0 {
					continue
				}
				_, errs := imp.check("main", files)
				with := ""
				if tests {
					with = " (with the _test.gp files)"
				}
				for _, e := range errs {
					t.Errorf("%s%s", e, with)
				}
			}
		})
	}
}

// modulePath returns the module path in the gocpp.mod of dir, or "" when
// there is none.
func modulePath(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "gocpp.mod"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if p, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.TrimSpace(p)
		}
	}
	t.Fatalf("%s/gocpp.mod has no module line", dir)
	return ""
}
