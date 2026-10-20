package check

import (
	"go/ast"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// focusTree is a package of three files: the function bodies of a.gp are the ones that a focused
// check looks at, and each file has an error in a body.
var focusTree = map[string]string{
	"gocpp.mod": "module example.com/m\n",
	"a.gp":      "package main\n\nfunc a() int {\n\treturn xa\n}\n",
	"b.gp": "package main\n\nvar Value = func() int { return 1 }\n\nfunc b() int {\n\treturn xb\n}\n\n" +
		"func init() {\n\t_ = xinit\n}\n\n" +
		"type T struct{}\n\nfunc (T) M() int {\n\treturn xm\n}\n",
	"c.gp": "package main\n\nfunc main() {\n\t_ = xc\n}\n",
}

// loadFocusTree loads the package of focusTree.
func loadFocusTree(t *testing.T) (*load.Program, string) {
	t.Helper()
	dir := writeTree(t, focusTree)
	cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
	return load.Load(cfg, "."), dir
}

// focusErrors checks the loaded package with the focus (a name in focusTree, or "") at phase, and
// returns its messages and whether the package is Focused.
func focusErrors(
	t *testing.T, prog *load.Program, dir, focus string, phase int,
) (messages string, focused bool) {
	t.Helper()
	cfg := fakeProbeConfig(phase)
	if focus != "" {
		cfg.Focus = filepath.Join(dir, focus)
	}
	r := Check(prog, cfg)
	p := r.Package(prog.Roots[0])
	var b strings.Builder
	for _, d := range p.Errors.Diagnostics() {
		b.WriteString(filepath.Base(d.Pos.Filename) + ": " + d.Msg + "\n")
	}
	return b.String(), p.Focused
}

func TestStripOtherBodies(t *testing.T) {
	bodies := func(prog *load.Program) map[string]bool {
		got := make(map[string]bool)
		for _, f := range prog.Roots[0].Files {
			for _, decl := range f.AST.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					got[filepath.Base(f.Path)+" "+fn.Name.Name] = fn.Body != nil
				}
			}
		}
		return got
	}
	t.Run("the focus file keeps its bodies", func(t *testing.T) {
		prog, dir := loadFocusTree(t)
		if !stripOtherBodies(prog.Roots[0], filepath.Join(dir, "a.gp")) {
			t.Fatal("the file was not found")
		}
		if got := bodies(prog); !got["a.gp a"] {
			t.Errorf("a.gp lost its body: %v", got)
		}
	})
	t.Run("the others lose theirs", func(t *testing.T) {
		prog, dir := loadFocusTree(t)
		stripOtherBodies(prog.Roots[0], filepath.Join(dir, "a.gp"))
		got := bodies(prog)
		for _, name := range []string{"b.gp b", "b.gp M", "c.gp main"} {
			if got[name] {
				t.Errorf("%s keeps its body", name)
			}
		}
	})
	t.Run("init keeps its body", func(t *testing.T) {
		prog, dir := loadFocusTree(t)
		stripOtherBodies(prog.Roots[0], filepath.Join(dir, "a.gp"))
		if got := bodies(prog); !got["b.gp init"] {
			t.Errorf("init lost its body: %v", got)
		}
	})
	t.Run("a file that is not in the package changes nothing", func(t *testing.T) {
		prog, dir := loadFocusTree(t)
		if stripOtherBodies(prog.Roots[0], filepath.Join(dir, "none.gp")) {
			t.Fatal("the file was found")
		}
		for name, has := range bodies(prog) {
			if !has {
				t.Errorf("%s lost its body", name)
			}
		}
	})
}

func TestFocusedCheck(t *testing.T) {
	prog, dir := loadFocusTree(t)
	full, focused := focusErrors(t, prog, dir, "", LastPhase)
	if focused {
		t.Fatal("a check without a focus is Focused")
	}
	wanted := []string{"undefined: xa", "undefined: xb", "undefined: xc", "undefined: xm", "undefined: xinit"}
	for _, want := range wanted {
		if !strings.Contains(full, want) {
			t.Fatalf("the full check lacks %q:\n%s", want, full)
		}
	}
	prog, dir = loadFocusTree(t)
	got, focused := focusErrors(t, prog, dir, "a.gp", LastPhase)

	t.Run("it is Focused", func(t *testing.T) {
		if !focused {
			t.Error("not Focused")
		}
	})
	t.Run("it reports the focus file's bodies", func(t *testing.T) {
		if !strings.Contains(got, "a.gp: undefined: xa") {
			t.Errorf("no error in a.gp:\n%s", got)
		}
	})
	t.Run("it leaves the other bodies out", func(t *testing.T) {
		for _, name := range []string{"xb", "xc", "xm"} {
			if strings.Contains(got, "undefined: "+name) {
				t.Errorf("an error in the body of another file:\n%s", got)
			}
		}
	})
	t.Run("it keeps the body of init", func(t *testing.T) {
		if !strings.Contains(got, "b.gp: undefined: xinit") {
			t.Errorf("no error in init:\n%s", got)
		}
	})
}

func TestFocusedUnknownFile(t *testing.T) {
	prog, dir := loadFocusTree(t)
	full, _ := focusErrors(t, prog, dir, "", LastPhase)
	prog, dir = loadFocusTree(t)
	got, focused := focusErrors(t, prog, dir, "none.gp", LastPhase)
	if focused {
		t.Error("Focused")
	}
	if got != full {
		t.Errorf("a focus on a file that is not there changed the errors:\n%s\nwant:\n%s", got, full)
	}
}

func TestFocusedGatesLookAtTheFocusFileOnly(t *testing.T) {
	// The closure in the initializer of Value is in a declaration, which a focused check keeps;
	// closures are gated until Phase 03.
	const gate = "b.gp: closures are not supported yet (Phase 03)"
	prog, dir := loadFocusTree(t)
	full, _ := focusErrors(t, prog, dir, "", 2)
	if !strings.Contains(full, gate) {
		t.Fatalf("the gate does not report the closure of b.gp:\n%s", full)
	}
	prog, dir = loadFocusTree(t)
	got, _ := focusErrors(t, prog, dir, "a.gp", 2)
	if strings.Contains(got, gate) {
		t.Errorf("a focused check on a.gp reports a gate in b.gp:\n%s", got)
	}
	prog, dir = loadFocusTree(t)
	got, _ = focusErrors(t, prog, dir, "b.gp", 2)
	if !strings.Contains(got, gate) {
		t.Errorf("a focused check on b.gp lacks its gate:\n%s", got)
	}
}
