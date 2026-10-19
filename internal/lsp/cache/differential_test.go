package cache

import (
	"context"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// TestSameAsCheck checks that the pipeline, which checks each package alone
// with its imports from summaries and export data, finds the diagnostics
// that a check of the whole program finds (load.Load and check.Check, as
// gocpp check does): for the packages of the examples, the programs of
// tests/run and tests/errors, and every standard package.
func TestSameAsCheck(t *testing.T) {
	var dirs []string // folders of packages to compare, each a program
	examples, _ := filepath.Glob("../../../examples/*/main.gp")
	for _, f := range examples {
		dirs = append(dirs, filepath.Dir(f))
	}
	tmp := t.TempDir()
	for _, folder := range []string{"run", "errors"} {
		files, _ := filepath.Glob("../../../tests/" + folder + "/*.gp")
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(tmp, folder, strings.TrimSuffix(filepath.Base(f), ".gp"))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, filepath.Base(f)), src, 0o644); err != nil {
				t.Fatal(err)
			}
			dirs = append(dirs, dir)
		}
	}
	// Programs of more than one package, whose packages share what the
	// summaries carry: opaque values (check.Dep.HasC), generics, dot
	// imports, and errors in an import.
	for name, files := range multiPackage {
		root := filepath.Join(tmp, "multi", name)
		for file, src := range files {
			path := filepath.Join(root, filepath.FromSlash(file))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		dirs = append(dirs, root)
	}
	if len(dirs) < 40 {
		t.Fatalf("only %d programs", len(dirs))
	}

	cfg := testConfig()
	p := NewPipeline(cfg)
	st := p.NewState().(*state)
	print := func(lists ...*diag.List) string {
		var b strings.Builder
		pr := diag.Printer{All: true}
		for _, l := range lists {
			if err := pr.Print(&b, l); err != nil {
				t.Fatal(err)
			}
		}
		return b.String()
	}
	// byFile adds the diagnostics of lists that have a place to m, by file
	// (an editor shows them by file: the pipeline reports an error of an
	// import at the importing spec of each importer, a check of the whole
	// program in the list of the import, at the first importing spec).
	byFile := func(m map[string][]string, lists ...*diag.List) {
		for _, l := range lists {
			for _, d := range l.Diagnostics() {
				if d.Pos.Filename != "" {
					var b strings.Builder
					one := new(diag.List)
					one.Add(d)
					if err := (&diag.Printer{All: true}).Print(&b, one); err != nil {
						t.Fatal(err)
					}
					m[d.Pos.Filename] = append(m[d.Pos.Filename], b.String())
				}
			}
		}
	}
	// placeless returns the diagnostics of lists that have no place, as
	// gocpp check prints them.
	placeless := func(lists ...*diag.List) string {
		l := new(diag.List)
		for _, list := range lists {
			for _, d := range list.Diagnostics() {
				if d.Pos.Filename == "" {
					l.Add(d)
				}
			}
		}
		return print(l)
	}
	compared := 0
	for _, dir := range dirs {
		dir, err := filepath.Abs(dir)
		if err != nil {
			t.Fatal(err)
		}
		prog := load.Load(load.Config{Dir: dir, Stdlib: cfg.Stdlib, GoStd: cfg.GoStd, GOOS: cfg.GOOS, GOARCH: cfg.GOARCH,
			AllErrors: true, NoEmbedFiles: true}, ".")
		r := check.Check(prog, check.Config{Phase: cfg.Phase, GOOS: cfg.GOOS, GOARCH: cfg.GOARCH, CXX: cfg.CXX})
		want, got := make(map[string][]string), make(map[string][]string)
		byFile(want, prog.Errors)
		for _, cp := range r.Packages {
			lp := cp.Load
			var h *handle
			if lp.Std {
				h, err = st.handle(context.Background(), pkgID{dir: lp.Dir, std: true}, lp.ImportPath, nil)
			} else {
				h, err = st.handle(context.Background(), pkgID{dir: lp.Dir}, "", nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := p.packageOf(context.Background(), h)
			// The namespace probe and its compiler lookup run in the
			// background (probe.go): compare once they are done.
			for try := 0; err == nil && (pkg.Provisional() || pkg.Stale()) && try < 10; try++ {
				p.prober.wait()
				pkg, err = p.packageOf(context.Background(), h)
			}
			if err != nil {
				t.Fatalf("%s: %v", lp.ImportPath, err)
			}
			byFile(want, cp.Errors)
			byFile(got, pkg.Program.Errors, pkg.Check.Errors)

			// The diagnostics without a place, per package. The known
			// difference: an error about a package as a whole (no .gp
			// files, ...) is at the first import in a check of the whole
			// program, but without a place in the pipeline, which loads
			// every package as a root (and reports it at its importing
			// specs, reportImportErrors: compared by file above).
			wantLists := []*diag.List{cp.Errors}
			if lp == prog.Roots[0] {
				wantLists = append(wantLists, prog.Errors)
			}
			atImport := new(diag.List)
			for _, d := range cp.Errors.Diagnostics() {
				if d.Pos.Filename != "" && filepath.Dir(d.Pos.Filename) != lp.Dir {
					d.Pos = token.Position{}
					atImport.Add(d)
				}
			}
			wantLists = append(wantLists, atImport)
			if g, w := placeless(pkg.Program.Errors, pkg.Check.Errors), placeless(wantLists...); g != w {
				t.Errorf("%s: the diagnostics without a place:\n%s\nwant:\n%s", lp.ImportPath, g, w)
			}
			if pkg.Check.Checked() != cp.Checked() {
				t.Errorf("%s: checked %v, want %v", lp.ImportPath, pkg.Check.Checked(), cp.Checked())
			}
			compared++
		}
		for file := range maps.Keys(want) {
			if _, ok := got[file]; !ok {
				got[file] = nil
			}
		}
		for file, ds := range got {
			slices.Sort(ds)
			w := want[file]
			slices.Sort(w)
			if !slices.Equal(ds, w) {
				t.Errorf("%s:\n%s\nwant (gocpp check):\n%s", file, strings.Join(ds, ""), strings.Join(w, ""))
			}
		}
	}
	// Every standard package, also those that no program imports.
	entries, err := stdPackages()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range entries {
		if slices.Contains(stdlib.WindowsOnly, path) {
			continue // no file for the default system (Linux): "build constraints exclude all .gp files"
		}
		h, err := st.handle(context.Background(), pkgID{dir: filepath.Join(p.cfg.StdlibDir, path), std: true}, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := p.packageOf(context.Background(), h)
		if err != nil {
			t.Fatal(err)
		}
		if errs := print(pkg.Program.Errors, pkg.Check.Errors); errs != "" || !pkg.Check.Checked() {
			t.Errorf("%s: errors:\n%s", path, errs)
		}
	}
	t.Logf("compared %d packages of %d programs and %d standard packages", compared, len(dirs), len(entries))
}

// stdPackages returns the import paths of the standard packages: the
// folders of stdlib.FS with .gp files.
func stdPackages() ([]string, error) {
	var paths []string
	err := fs.WalkDir(stdlib.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".gp") {
			if dir := filepath.ToSlash(filepath.Dir(path)); !slices.Contains(paths, dir) {
				paths = append(paths, dir)
			}
		}
		return err
	})
	return paths, err
}

// multiPackage are the programs of more than one package of TestSameAsCheck.
var multiPackage = map[string]map[string]string{
	// The opaque values of a reach main through b (TestConversionOtherPackage
	// of package check).
	"opaque": {
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\n\nvar X = C.f()\n\nfunc F() *C.FILE { return nil }\n",
		"b/b.gp":    "package b\n\nimport \"example.com/m/a\"\n\nvar Y = a.X\n",
		"main.gp": `package main

import (
	"example.com/m/a"
	"example.com/m/b"
)

func main() {
	n := int(a.X)
	_ = n + "a"
	k := int(*a.F())
	_ = k + "a"
	y := int(b.Y)
	_ = y + "a"
}
`,
	},
	"generics": {
		"gocpp.mod": "module example.com/g\n",
		"list/list.gp": `package list

type List[T any] struct{ items []T }

func (l *List[T]) Push(v T) { l.items = append(l.items, v) }

func Map[T, U any](l *List[T], f func(T) U) *List[U] {
	r := &List[U]{}
	for _, v := range l.items {
		r.Push(f(v))
	}
	return r
}

type Number interface{ ~int | ~float64 }

func Sum[T Number](xs ...T) (s T) {
	for _, x := range xs {
		s += x
	}
	return
}
`,
		"main.gp": `package main

import (
	"fmt"
	"strconv"

	. "example.com/g/list"
)

type Meters float64

func main() {
	l := &List[int]{}
	l.Push(1)
	s := Map(l, strconv.Itoa)
	s.Push(2)
	fmt.Println(Sum(Meters(1), 2.5), Sum("a"))
}
`,
	},
	// Errors about an imported package as a whole, which a check of the
	// whole program reports at the import.
	"nogpfiles": {
		"gocpp.mod":  "module example.com/x\n",
		"lib/README": "no code\n",
		"main.gp":    "package main\n\nimport \"example.com/x/lib\"\n\nfunc main() { lib.F() }\n",
	},
	"onlytests": {
		"gocpp.mod":       "module example.com/x\n",
		"lib/lib_test.gp": "package lib\n",
		"main.gp":         "package main\n\nimport \"example.com/x/lib\"\n\nfunc main() { lib.F() }\n",
	},
	"constraints": {
		"gocpp.mod":          "module example.com/x\n",
		"lib/lib_windows.gp": "package lib\n\nfunc F() {}\n",
		"main.gp":            "package main\n\nimport \"example.com/x/lib\"\n\nfunc main() { lib.F() }\n",
	},
	"twopackages": {
		"gocpp.mod": "module example.com/x\n",
		"lib/a.gp":  "package lib\n\nfunc F() {}\n",
		"lib/b.gp":  "package other\n",
		"main.gp":   "package main\n\nimport \"example.com/x/lib\"\n\nfunc main() { lib.F() }\n",
	},
	"brokenimport": {
		"gocpp.mod":  "module example.com/x\n",
		"lib/lib.gp": "package lib\n\nfunc F() int { return \"s\" }\n\nvar V undefinedType\n",
		"main.gp":    "package main\n\nimport \"example.com/x/lib\"\n\nfunc main() { var s string = lib.F(); _ = s; lib.V.M() }\n",
	},
}

// TestImportErrorsAtEverySpec checks the known difference of the import
// errors: an error about an imported package as a whole is at every spec
// that imports it, in every importer (a check of the whole program has it
// once, at the first import).
func TestImportErrorsAtEverySpec(t *testing.T) {
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod":  "module example.com/x\n",
		"lib/README": "no code\n",
		"a.gp":       "package main\n\nimport (\n\t\"example.com/x/lib\"\n\t\"example.com/x/other\"\n)\n\nfunc main() { lib.F(); other.G() }\n",
		"b.gp":       "package main\n\nimport \"example.com/x/lib\"\n\nvar _ = lib.F\n",
		"other/o.gp": "package other\n\nimport \"example.com/x/lib\"\n\nfunc G() { lib.F() }\n",
	})
	lib := filepath.Join(w.root, "lib")
	if got, want := w.errs(w.pkg(".")), "./a.gp:4:2: no .gp files in "+lib+"\n./b.gp:3:8: no .gp files in "+lib+"\n"; got != want {
		t.Errorf("main: errors\n%s\nwant\n%s", got, want)
	}
	if got, want := w.errs(w.pkg("other")), "other/o.gp:3:8: no .gp files in "+lib+"\n"; got != want {
		t.Errorf("other: errors\n%s\nwant\n%s", got, want)
	}
	if w.pkg(".").Check.Checked() || w.pkg("other").Check.Checked() {
		t.Error("an importer of lib was type-checked")
	}
}
