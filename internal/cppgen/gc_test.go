package cppgen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
)

// emitGoSource parses, checks and emits one Go source file for the GC engine (job 16.1c).
func emitGoSource(t *testing.T, src string) (string, error) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	files := []*ast.File{f}
	pkg, info, err := check.CheckGo(fset, files)
	if err != nil {
		return "", err
	}
	out, err := EmitGo(fset, pkg, info, files, GCConfig{OutputPath: "/tmp/gopkg_main/main.cpp"})
	return string(out), err
}

// TestEmitGo checks the shape of the walking skeleton's C++: the namespace, Go's layout, the
// prototypes (Go lets a function be called before its declaration), #line directives, and the
// plain int main.
func TestEmitGo(t *testing.T) {
	const src = `package main

func main() {
	println(later(2), "x")
}

func later(n int) int {
	return n + 1
}
`
	out, err := emitGoSource(t, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#include <gocpp/gc/rt.hpp>",
		"namespace gopkg_main {",
		"int64_t later(int64_t n);", // the prototype
		"void main() {",
		"auto gp_t0 = later(int64_t(2));", // the call is hoisted: Go's order is its own
		`gp::gc::println(gp_t0, gp::gc::lit("x"));`,
		"int main() {",
		"gp::gc::init();", // the collector's process setup, before the package's main (job 16.1d)
		"gopkg_main::main();",
		`#line 3 "main.go"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the C++ lacks %q:\n%s", want, out)
		}
	}
	// The collector is initialized before the package's main can allocate.
	if init, main := strings.Index(out, "gp::gc::init();"), strings.Index(out, "gopkg_main::main();"); init < 0 || main < 0 || init > main {
		t.Errorf("gp::gc::init() is not before gopkg_main::main():\n%s", out)
	}
	if proto, def := strings.Index(out, "int64_t later(int64_t n);"), strings.Index(out, "int64_t later(int64_t n) {"); proto < 0 || def < 0 || proto > def {
		t.Errorf("the prototype of later is not before its definition:\n%s", out)
	}
}

// TestEmitGoOrder checks Go's evaluation order: the arguments of a call and the operands of a
// binary expression run left to right, through temporaries, and Go's shifts and division are the
// gp::gc helpers (job 16.1c, review round 2).
func TestEmitGoOrder(t *testing.T) {
	const src = `package main

func next(n int) int {
	return n
}

func pair(a, b int) int {
	return a*10 + b
}

func main() {
	println(pair(next(1), next(2)))
	println(7 << 3, 7 / 2, 7 % 2)
	println(len("abcd"))
	println(-(9223372036854775808), 0-9223372036854775808)
}
`
	out, err := emitGoSource(t, src)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.Index(out, "auto gp_t0 = next(int64_t(1));")
	second := strings.Index(out, "auto gp_t1 = next(int64_t(2));")
	call := strings.Index(out, "pair(gp_t0, gp_t1)")
	if first < 0 || second < 0 || call < 0 || !(first < second && second < call) {
		t.Errorf("the call arguments are not sequenced left to right (t0 %d, t1 %d, call %d):\n%s", first, second, call, out)
	}
	// len has no side effect: it is not hoisted into a temporary.
	if !strings.Contains(out, `gp::gc::println(gp::gc::len(gp::gc::lit("abcd")));`) {
		t.Errorf("len is not left in place (it is pure):\n%s", out)
	}
	// -2^63 is INT64_MIN in every form, never a too-large literal.
	if strings.Count(out, "INT64_MIN") != 2 {
		t.Errorf("the two -2^63 forms are not both INT64_MIN:\n%s", out)
	}
	for _, want := range []string{
		"gp::gc::shl(int64_t(7), int64_t(3))",
		"gp::gc::div(int64_t(7), int64_t(2))",
		"gp::gc::rem(int64_t(7), int64_t(2))",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the C++ lacks %q:\n%s", want, out)
		}
	}
}

// TestEmitGoTypes checks Go's layout of the three types of the subset (README §2.2).
func TestEmitGoTypes(t *testing.T) {
	const src = `package main

func f(s string, b bool) string {
	return s
}

func main() {
	var ok bool
	var n int
	println(f("x", ok), n)
}
`
	out, err := emitGoSource(t, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"gp::gc::string f(gp::gc::string s, bool b) {",
		"bool ok{};",
		"int64_t n{};",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the C++ lacks %q:\n%s", want, out)
		}
	}
}

// TestEmitGoErrors checks that a construct outside the subset is an error with its .go
// position, not silently different C++.
func TestEmitGoErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{"switch", "package main\n\nfunc main() {\n\tswitch 1 {\n\t}\n}\n", "does not support a switch statement"},
		{"float", "package main\n\nfunc main() {\n\tx := 1.5\n\tprintln(x)\n}\n", "supports only int, string and bool"},
		{"type", "package main\n\ntype T int\n\nfunc main() {}\n", "does not support type declarations"},
		{"init", "package main\n\nfunc init() {}\n\nfunc main() {}\n", "does not support func init"},
		{"main signature", "package main\n\nfunc main(x int) {\n\tprintln(x)\n}\n", "main must have no"},
		{"right side effect", "package main\n\nfunc t() bool { return true }\n\nfunc main() {\n\tif true && t() {\n\t}\n}\n", "side effect on the right of &&"},
		{"huge div", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 / 2)\n}\n", "cannot represent the integer literal"},
		{"huge rem", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 % 3)\n}\n", "cannot represent the integer literal"},
		{"huge shr", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 >> 1)\n}\n", "cannot represent the integer literal"},
		{"huge sub", "package main\n\nfunc main() {\n\tprintln(9223372036854775808 - 1)\n}\n", "cannot represent the integer literal"},
		{"huge 2^64", "package main\n\nfunc main() {\n\tprintln(18446744073709551616 - 18446744073709551616)\n}\n", "cannot represent the integer literal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := emitGoSource(t, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("EmitGo = %v, want an error containing %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), "main.go:") {
				t.Errorf("the error has no .go position: %v", err)
			}
		})
	}
}

// TestGCNamespace checks the gopkg_ rule: the escaping of gocpp's gpkg_ names with the GC
// engine's prefix (README §2.2, ADR-010).
func TestGCNamespace(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"main", "gopkg_main"},
		{"example.com/app/geom", "gopkg_example_dcom__app__geom"},
		{"github.com/gin-gonic/gin", "gopkg_github_dcom__gin_hgonic__gin"},
	} {
		if got := gcNamespace(tc.path); got != tc.want {
			t.Errorf("gcNamespace(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
