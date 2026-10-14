package check

import (
	"strings"
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
