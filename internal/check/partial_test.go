package check

import (
	"go/ast"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// TestPartial: with Config.Partial, a package with load errors gets the
// types that go/types finds anyway (Package.Partial), and nothing else
// changes: the errors, Checked, and what its importers get (Result.Dep).
func TestPartial(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\n\nimport \"example.com/m/nonexist\"\n\nvar X = nonexist.X\nvar Y int = 1\n",
		"d/d.gp":    "package d\n\nfunc F() int { return 1 }\n\nfunc g() { x := }\n",
		"main.gp":   "package main\n\nimport (\n\t\"example.com/m/a\"\n\t\"example.com/m/d\"\n)\n\nfunc main() { _, _ = a.Y, d.F() }\n",
	})
	cfg := load.Config{}
	prog, r := loadAndCheck(t, cfg, root, ".")
	want := errorText(t, prog, r, root, true)

	prog, r = loadAndCheckPartial(t, cfg, root)
	if got := errorText(t, prog, r, root, true); got != want {
		t.Errorf("errors with Partial:\n%s\nwant:\n%s", got, want)
	}
	for _, cp := range r.Packages {
		if cp.Load.Std {
			continue
		}
		if cp.Checked() || cp.Partial == nil || cp.Partial.Types == nil || cp.Partial.Info == nil {
			t.Fatalf("%s: Checked %v, Partial %+v; want a partial package", cp.Load.ImportPath, cp.Checked(), cp.Partial)
		}
		if dep, _ := r.Dep(cp.Load); dep.Types != nil {
			t.Errorf("%s: Dep has types", cp.Load.ImportPath)
		}
		scope := cp.Partial.Types.Scope()
		switch cp.Load.ImportPath {
		case "example.com/m/a":
			if y := scope.Lookup("Y"); y == nil || y.Type().String() != "int" {
				t.Errorf("a.Y: %v", y)
			}
		case "example.com/m/d":
			if f := scope.Lookup("F"); f == nil || !strings.HasPrefix(f.Type().String(), "func() int") {
				t.Errorf("d.F: %v", f)
			}
		case "main":
			// The imports of packages with errors are missing.
			if m := scope.Lookup("main"); m == nil {
				t.Errorf("main.main is missing")
			}
		}
	}
}

// TestLazyPartialMustSynthetic: diagnostics need no partial rewrite/check; concurrent feature requests coalesce it,
// and every identifier that the lazy ! preparation introduced is marked synthetic, never exposed as user code.
func TestLazyPartialMustSynthetic(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp":      "package a\n\nimport \"strconv\"\n\nfunc good(s string) (int, error) { n := strconv.Atoi!(s); return n, nil }\n",
		"broken.gp": "package a\n\nfunc broken() { x := }\n",
	})
	cfg := load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
	prog := load.Load(cfg, ".")
	lp := prog.Roots[0]
	user := identsOf(lp)
	ccfg := fakeProbeConfig(LastPhase)
	ccfg.Partial, ccfg.LazyPartial = true, true
	r := Check(prog, ccfg)
	p := r.Package(lp)
	if !p.HasPartial() || p.Partial != nil || p.MustPartial != nil || p.Must != nil || p.Checked() {
		t.Fatalf("partial work was not lazy: %+v", p)
	}
	before := len(p.Errors.Diagnostics())
	parts := make([]*Partial, 8)
	var wg sync.WaitGroup
	for i := range parts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if !p.HasPartial() {
				return
			}
			parts[i] = p.PartialResult()
			p.MustPartialInfo()
		}(i)
	}
	wg.Wait()
	for _, part := range parts {
		if part == nil || part != parts[0] {
			t.Fatalf("partial requests did not coalesce")
		}
	}
	if p.Must != nil || p.MustPartialInfo() == nil || len(p.MustPartialInfo().Calls) != 1 || len(p.Errors.Diagnostics()) != before {
		t.Fatalf("partial changed compiler info/diagnostics or lost the written ! call")
	}
	synthetic := 0
	for _, f := range lp.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && !user[id] {
				synthetic++
				if !f.Synthetic[id] {
					t.Errorf("unmarked synthetic identifier %q", id.Name)
				}
			}
			return true
		})
	}
	if synthetic == 0 {
		t.Fatal("no synthetic identifiers were prepared")
	}
}

// loadAndCheckPartial is loadAndCheck of "." with Config.Partial.
func loadAndCheckPartial(t *testing.T, cfg load.Config, dir string) (*load.Program, *Result) {
	t.Helper()
	cfg.Dir, cfg.Stdlib, cfg.GoStd = dir, stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	prog := load.Load(cfg, ".")
	ccfg := fakeProbeConfig(LastPhase)
	ccfg.Partial = true
	return prog, Check(prog, ccfg)
}
