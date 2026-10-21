package check

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// TestExternFuncs checks the extern functions (ADR-128, externfuncs.go): the signature rules, the
// uses that are not calls (also of a function of another package), and that a call of an extern
// function type-checks like any call.
func TestExternFuncs(t *testing.T) {
	lib := `package lib

//gocpp:extern C
func Add3(a, b, c C.int) C.int

//gocpp:extern C
func Bad(s string) C.int

func Body(x int) int { return x }
`
	src := `package main

import "example.com/m/lib"

//gocpp:extern C
func local(p *C.char) C.int

//gocpp:extern
func vari(f *C.char, rest ...C.int)

func main() {
	_ = lib.Add3(1, 2, 3)
	_ = local(nil)
	f := lib.Add3
	g := local
	_, _ = f, g
	h := lib.Body
	_ = h
}
`
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"lib/lib.gp": lib,
		"main.gp":    src,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m/lib\n" +
		"lib/lib.gp:7:12: extern function Bad: parameter type string is a Go type that C++ does not have (use a C type such as *C.char, or a number type of Go)\n" +
		"# example.com/m\n" +
		"./main.gp:9:27: extern function vari can not be variadic: C varargs are not supported (declare a C wrapper with a fixed list of parameters, written in a C file of the package)\n" +
		"./main.gp:14:7: cannot use extern function lib.Add3 as a value: it is a C or C++ function, only a call is possible (wrap it in a Go function to pass it on)\n" +
		"./main.gp:15:7: cannot use extern function local as a value: it is a C or C++ function, only a call is possible (wrap it in a Go function to pass it on)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
}

// TestExternSet: the set holds the extern functions of the package and of its imports, by
// import path and name.
func TestExternSet(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"lib/lib.gp": "package lib\n\n//gocpp:extern C\nfunc Add3(a, b, c C.int) C.int\n\nfunc Plain() {}\n",
		"main.gp":    "package main\n\nimport \"example.com/m/lib\"\n\n//gocpp:extern C\nfunc local()\n\nfunc main() { lib.Plain(); local(); _ = lib.Add3(1, 2, 3) }\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	p := mainPkg(t, prog, r)
	set := ExternSet(p.Load)
	for _, name := range []string{"example.com/m.local", "example.com/m/lib.Add3"} {
		if !set[name] {
			t.Errorf("%s is missing from %v", name, set)
		}
	}
	if len(set) != 2 {
		t.Errorf("got %v, want the two extern functions only", set)
	}
}

// TestExternSignatureRules: one declaration per case; want is a part of the one error, or "".
func TestExternSignatureRules(t *testing.T) {
	tests := []struct {
		name string
		decl string
		want string
	}{
		{name: "sized numbers and C types", decl: "func f(a int32, b uint8, c float64, d C.int, e *C.char) C.long"},
		{name: "unsafe.Pointer and a pointer to a number as parameters", decl: "func f(p unsafe.Pointer, out *int32)"},
		{name: "a pointer to a C type as the result", decl: "func f() *C.char"},
		{name: "int parameter", decl: "func f(n int)",
			want: "parameter type int is the Go int (64 bits), which is not the C int (32 bits): write C.int, int32, int64, C.long or C.longlong"},
		{name: "uint parameter", decl: "func f(n uint)",
			want: "parameter type uint is the Go uint (64 bits), which is not the C unsigned int (32 bits): write uint32, uint64, C.ulong or C.size_t"},
		{name: "pointer to int", decl: "func f(n *int)", want: "parameter type *int is the Go int"},
		{name: "int result", decl: "func f() int", want: "result type int is the Go int"},
		{name: "unsafe.Pointer result", decl: "func f() unsafe.Pointer",
			want: "result type unsafe.Pointer can not be the result of a C function: a gocpp pointer is a managed object"},
		{name: "pointer to a number as the result", decl: "func f() *int32",
			want: "result type *int32 can not be the result of a C function"},
		{name: "pointer to a pointer to a number as the result", decl: "func f() **uint8",
			want: "result type **uint8 can not be the result of a C function"},
		{name: "two results", decl: "func f() (C.int, C.int)", want: "extern function f returns more than one value"},
		{name: "two named results", decl: "func f() (a, b C.int)", want: "extern function f returns more than one value"},
		{name: "one named result", decl: "func f() (a C.int)"},
		{name: "variadic", decl: "func f(args ...C.int)", want: "extern function f can not be variadic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package main\n\nimport \"unsafe\"\n\nvar _ unsafe.Pointer\n\n//gocpp:extern C\n" + tt.decl + "\n\nfunc main() {}\n"
			root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
			prog, r := loadAndCheck(t, load.Config{}, root, ".")
			got := errorText(t, prog, r, root, true)
			switch {
			case tt.want == "" && got != "":
				t.Errorf("unexpected errors:\n%s", got)
			case tt.want != "" && !strings.Contains(got, tt.want):
				t.Errorf("errors:\n%s\nwant one with %q", got, tt.want)
			case tt.want != "" && strings.Count(got, "\n") > 2:
				t.Errorf("more than one error:\n%s", got)
			}
		})
	}
}

// TestBodylessFunctions: a function without a body needs the directive, in a package of the
// project (go/types reports only a generic function).
func TestBodylessFunctions(t *testing.T) {
	const want = "./main.gp:3:6: missing function body (to declare a function that C or C++ defines, put //gocpp:extern in its doc comment)"
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\nfunc noBody(x int32) int32\n\n//gocpp:extern C\nfunc withDirective(x int32) int32\n\nfunc main() { _ = withDirective(noBody(1)) }\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	got := errorText(t, prog, r, root, true)
	if !strings.Contains(got, want) || strings.Count(got, "missing function body") != 1 {
		t.Errorf("errors:\n%s\nwant exactly one: %s", got, want)
	}
}

// TestBodylessMethod: a method without a body is the same error as a function without one (go/types
// does not report it, and the build stopped with an internal error); the directive stays a
// function's own, so a method that has it keeps its own message.
func TestBodylessMethod(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": "package main\n\ntype T struct{}\n\nfunc (T) noBody()\n\n" +
			"func (*T) pointerNoBody(x int)\n\ntype G[P any] struct{}\n\nfunc (G[P]) genericNoBody()\n\n" +
			"func (T) hasBody() {}\n\nfunc main() {}\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	got := errorText(t, prog, r, root, true)
	for _, line := range []string{"main.gp:5:10: missing function body", "main.gp:7:11: missing function body",
		"main.gp:11:13: missing function body"} {
		if !strings.Contains(got, line) {
			t.Errorf("errors:\n%s\nwant one starting %s", got, line)
		}
	}
	if strings.Count(got, "missing function body") != 3 {
		t.Errorf("errors:\n%s\nwant exactly three", got)
	}
}

// TestAllowBodyless: Config.AllowBodyless (the runner of Go's own test files) turns the error off.
func TestAllowBodyless(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\ntype T struct{}\n\nfunc (T) m()\n\nfunc f()\n\nfunc main() {}\n",
	})
	prog := load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	cfg := fakeProbeConfig(LastPhase)
	cfg.AllowBodyless = true
	r := Check(prog, cfg)
	if got := errorText(t, prog, r, root, true); got != "" {
		t.Errorf("errors with AllowBodyless:\n%s", got)
	}
}

// TestBodylessFunctionsInBindgenView: the view that gocpp bindgen writes (ADR-092) describes a
// header with functions that have no body, and gocpp check accepts it.
func TestBodylessFunctionsInBindgenView(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"view.gp":   "// Code generated by gocpp bindgen from <lib.h>. DO NOT EDIT.\n\npackage lib\n\nfunc add(a C.int, b C.int) C.int\n",
		"other.gp":  "package lib\n\nfunc sub(a C.int, b C.int) C.int\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	got := errorText(t, prog, r, root, true)
	if strings.Contains(got, "view.gp") || !strings.Contains(got, "other.gp:3:6: missing function body") {
		t.Errorf("errors:\n%s\nwant the missing body of other.gp only", got)
	}
}
