package check

import (
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// TestDeps checks a program one package at a time, as the language server
// does (Phase 11, step 11.3): each package is loaded with the summaries of
// the packages it imports (load.Config.Deps) and checked with what the
// checks before said about them (Config.Deps from Result.Dep). The result
// is the one of a check of the whole program (TestConversionOtherPackage):
// the opaque values of a reach b and main through Dep.HasC.
func TestDeps(t *testing.T) {
	root := writeTree(t, map[string]string{
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
	})
	summaries := make(map[string]*load.Package) // by import path
	deps := make(map[*load.Package]Dep)
	checkOne := func(pattern string) (*load.Program, *Result) {
		cfg := load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64",
			Deps: func(importPath, _ string, _ bool) *load.Package { return summaries[importPath] }}
		prog := load.Load(cfg, pattern)
		if len(prog.Packages) != 1 {
			t.Fatalf("%s: %d packages loaded, want 1", pattern, len(prog.Packages))
		}
		ccfg := fakeProbeConfig(LastPhase)
		ccfg.Deps = func(lp *load.Package) Dep {
			d, ok := deps[lp]
			if !ok {
				t.Fatalf("%s: Deps(%s): not a summary", pattern, lp.ImportPath)
			}
			return d
		}
		r := Check(prog, ccfg)
		lp := prog.Roots[0]
		s := lp.Summary()
		summaries[lp.ImportPath] = s
		d, ok := r.Dep(lp)
		if !ok || d.Types == nil {
			t.Fatalf("%s: Dep %+v, %v", pattern, d, ok)
		}
		deps[s] = d
		return prog, r
	}
	_, ra := checkOne("./a")
	if d, _ := ra.Dep(ra.Packages[0].Load); !d.HasC || d.Failed {
		t.Errorf("a: Dep %+v, want HasC", d)
	}
	_, rb := checkOne("./b")
	if d, _ := rb.Dep(rb.Packages[0].Load); !d.HasC || d.Failed {
		t.Errorf("b: Dep %+v, want HasC from a", d)
	}
	prog, r := checkOne(".")
	want := "# example.com/m\n" +
		"./main.gp:10:6: invalid operation: n + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:12:6: invalid operation: k + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:14:6: invalid operation: y + \"a\" (mismatched types int and untyped string)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	if p := mainPkg(t, prog, r); len(p.Conversions) != 3 {
		t.Errorf("main: %d conversions, want 3", len(p.Conversions))
	}

	// A package without types fails its importers' imports, like an
	// incomplete package, and counts as Failed.
	prog = load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64",
		Deps: func(importPath, _ string, _ bool) *load.Package { return summaries[importPath] }}, "./b")
	ccfg := fakeProbeConfig(LastPhase)
	ccfg.Deps = func(*load.Package) Dep { return Dep{HasC: true} }
	r = Check(prog, ccfg)
	if d, _ := r.Dep(prog.Roots[0]); !d.Failed {
		t.Errorf("b over an a without types: Dep %+v, want Failed", d)
	}
	if errs := errorText(t, prog, r, root, true); errs == "" {
		t.Error("b over an a without types: no errors")
	}
}
