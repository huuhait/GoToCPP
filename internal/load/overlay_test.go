package load

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// The tests of the language server's hooks (Phase 11, step 11.3):
// Config.Overlay and Config.Deps with Package.Summary.

// TestOverlay checks that the files of Config.Overlay are read instead of
// the files on disk, also files and folders that are only in the overlay,
// and a gocpp.mod file.
func TestOverlay(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/app\n",
		"main.gp":      "package main\n\nimport \"example.com/app/geom\"\n\nfunc main() { geom.Old() }\n",
		"geom/geom.gp": "package geom\n\nfunc Old() {}\n",
	})
	overlay := map[string][]byte{
		// Shadows the file on disk: geom.New, not geom.Old.
		filepath.Join(root, "main.gp"): []byte("package main\n\nimport (\n\t\"example.com/app/geom\"\n\t\"example.com/app/shapes\"\n)\n\nfunc main() { geom.New(); shapes.Area() }\n"),
		// A file that is not on disk yet.
		filepath.Join(root, "geom", "new.gp"): []byte("package geom\n\nfunc New() {}\n"),
		// A folder that is not on disk yet.
		filepath.Join(root, "shapes", "shapes.gp"): []byte("package shapes\n\nfunc Area() {}\n"),
	}
	prog := loadIn(t, Config{Overlay: overlay}, root, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"example.com/app/geom geom: geom.gp new.gp",
		"example.com/app/shapes shapes: shapes.gp",
		"example.com/app main: main.gp; example.com/app/geom example.com/app/shapes",
	)
	main := prog.Roots[0]
	if got := main.Files[0].AST.Decls[1]; got == nil {
		t.Fatal("main.gp has no func main")
	}

	// A gocpp.mod file in the overlay gives the module path.
	overlay = map[string][]byte{filepath.Join(root, "gocpp.mod"): []byte("module example.com/other\n")}
	prog = loadIn(t, Config{Overlay: overlay}, root, ".")
	wantErrors(t, prog, root, `# example.com/other
./main.gp:3:8: no required module provides package example.com/app/geom
`)

	// The overlay of a file named on the command line.
	name := filepath.Join(root, "geom", "geom.gp")
	overlay = map[string][]byte{name: []byte("package geom\n\nfunc Changed() {}\n")}
	prog = loadIn(t, Config{Overlay: overlay}, root, name)
	if f := prog.Roots[0].Files[0]; prog.Roots[0].exportedNames()["Changed"] != true {
		t.Errorf("the named file %s was read from disk: %v", f.Path, prog.Roots[0].exportedNames())
	}
}

// TestDeps checks that a package that Config.Deps gives is used as it is:
// none of its files is read, it is not in Packages, its exported names
// work for a dot import, its Incomplete counts for its importers, and Load
// never changes it.
func TestDeps(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/app\n",
		"main.gp":      "package main\n\nimport (\n\t\"fmt\"\n\t\"<stdio.h>\"\n\n\t. \"example.com/app/geom\"\n)\n\nfunc main() { fmt.Println(Area(Rect{}), Origin.W, std.size_t(0), stdio.puts) }\n",
		"geom/geom.gp": "package geom\n\nimport \"fmt\"\n\ntype Rect struct{ W, H int }\n\nvar Origin Rect\n\nfunc Area(r Rect) int { return r.W * r.H }\n\nvar _ = fmt.Sprint\n",
	})
	// The summaries come from a load of geom alone.
	first := loadIn(t, Config{}, root, "./geom")
	wantErrors(t, first, root, "")
	geom := first.Roots[0].Summary()
	if !geom.summary || len(geom.Files) != 0 || len(geom.Imports) != 0 || geom.Name != "geom" ||
		!reflect.DeepEqual(geom.exported, map[string]bool{"Rect": true, "Origin": true, "Area": true}) {
		t.Fatalf("the summary of geom: %+v", geom)
	}
	before := *geom

	// The files of geom on disk are broken now: a load that reads them
	// would report it.
	if err := os.WriteFile(filepath.Join(root, "geom", "geom.gp"), []byte("package geom\n\nfunc {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var asked []string
	deps := func(importPath, dir string, std bool) *Package {
		asked = append(asked, importPath)
		if importPath == "example.com/app/geom" {
			if dir != filepath.Join(root, "geom") || std {
				t.Errorf("Deps(%s, %s, %v)", importPath, dir, std)
			}
			return geom
		}
		return nil // fmt: loaded by Load
	}
	prog := loadIn(t, Config{Deps: deps}, root, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog, "fmt fmt: print.gp", "example.com/app main: main.gp; example.com/app/geom fmt")
	if !reflect.DeepEqual(asked, []string{"fmt", "example.com/app/geom"}) {
		t.Errorf("Deps was asked for %v", asked)
	}
	main := prog.Roots[0]
	if main.Imports["example.com/app/geom"] != geom || main.Incomplete {
		t.Errorf("main: Imports %v, Incomplete %v", main.Imports, main.Incomplete)
	}
	// In a file with a header import, Origin.W would be a C++ namespace
	// use, but the dot import brings in Origin (from the names of the
	// summary): only std is a namespace.
	if ns := main.Files[0].Rewrite.Namespaces; len(ns) != 1 || ns[0].Name != "std" {
		t.Errorf("main.gp: namespaces %v, want only std", ns)
	}
	if !reflect.DeepEqual(*geom, before) {
		t.Errorf("Load changed the summary:\n%+v\nwas\n%+v", *geom, before)
	}

	// An incomplete summary makes its importers incomplete.
	broken := loadIn(t, Config{}, root, "./geom")
	if !broken.Roots[0].Incomplete {
		t.Fatal("the broken geom is complete")
	}
	bad := broken.Roots[0].Summary()
	prog = loadIn(t, Config{Deps: func(importPath, _ string, _ bool) *Package {
		if importPath == "example.com/app/geom" {
			return bad
		}
		return nil
	}}, root, ".")
	if !prog.Roots[0].Incomplete || len(prog.Roots[0].Errors.Diagnostics()) != 0 {
		t.Errorf("main with a broken geom: Incomplete %v, errors %v", prog.Roots[0].Incomplete, prog.Roots[0].Errors.Diagnostics())
	}

	// With Tests, Deps is not used.
	asked = nil
	loadIn(t, Config{Deps: deps, Tests: true}, root, ".")
	if len(asked) != 0 {
		t.Errorf("with Tests, Deps was asked for %v", asked)
	}
}

// mapFS is a FileSystem of files by absolute name; a folder is there when a
// file is in it or below it. It counts the reads.
type mapFS struct {
	files map[string]string
	reads int
}

func (m *mapFS) ReadFile(name string) ([]byte, error) {
	m.reads++
	if src, ok := m.files[name]; ok {
		return []byte(src), nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (m *mapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	m.reads++
	var entries []fs.DirEntry
	found := false
	for path := range m.files {
		if strings.HasPrefix(path, name+string(filepath.Separator)) {
			found = true
			if filepath.Dir(path) == name {
				entries = append(entries, overlayEntry(filepath.Base(path)))
			}
		}
	}
	if !found {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func (m *mapFS) Stat(name string) (fs.FileInfo, error) {
	m.reads++
	if _, ok := m.files[name]; ok {
		return fstest.MapFS{"f": {}}.Stat("f")
	}
	for path := range m.files {
		if strings.HasPrefix(path, name+string(filepath.Separator)) {
			return fstest.MapFS{"d/f": {}}.Stat("d")
		}
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// TestFS checks that with Config.FS, Load reads the files and folders,
// gocpp.mod included, only through it: the tree is not on disk.
func TestFS(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-on-disk")
	m := &mapFS{files: map[string]string{}}
	for name, src := range app {
		m.files[filepath.Join(root, filepath.FromSlash(name))] = src
	}
	prog := loadIn(t, Config{FS: m}, root, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"fmt fmt: print.gp",
		"example.com/app/geom geom: area.gp geom.gp; fmt",
		"unicode/utf8 utf8: utf8.gp",
		"strings strings: strings.gp; unicode/utf8",
		"example.com/app main: main.gp; example.com/app/geom fmt strings",
	)
	if m.reads == 0 {
		t.Error("nothing was read through the FS")
	}
}
