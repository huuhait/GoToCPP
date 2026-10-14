package cppgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// orderPrelude is the start of each TestOrderPanics program: f and grow change the package
// variables x and s, and print, so the output shows when they ran.
const orderPrelude = `package main

import "fmt"

var x = 1
var s = "abc"

func f() int {
	x++
	fmt.Println("f", x)
	return x
}

func grow() int {
	s += "z"
	fmt.Println("grow", len(s))
	return len(s)
}

func id(v int) int {
	fmt.Println("id", v)
	return v
}

`

// TestOrderPanics checks the order of calls and of the operations that can panic (order.go)
// against real Go: gc calls f first in a/b + f() and panics after it, and reads s after grow
// in int(s[i]) + grow(). Each program panics, with Go's output and exit code 2, on both
// compilers. (Which of two panicking operations without a call between them panics first is
// not kept: see order.go.)
func TestOrderPanics(t *testing.T) {
	tests := []struct{ name, decls, body string }{
		{"div-call", "", "a, b := 7, 0\n\tfmt.Println(a/b + f())"},
		{"call-div", "", "a, b := 7, 0\n\tfmt.Println(f() + a/b)"},
		{"call-over-zero", "", "z := 0\n\tfmt.Println(f() / z)"},
		{"div-decl", "", "a, b := 7, 0\n\ty, z := a/b, f()\n\tfmt.Println(y, z)"},
		{"div-var", "", "a, b := 7, 0\n\tvar y, z = a / b, f()\n\tfmt.Println(y, z)"},
		{"div-assign", "", "a, b := 7, 0\n\ty := 0\n\ty = a/b + f()\n\tfmt.Println(y)"},
		{"div-parallel", "", "a, b := 7, 0\n\ty, z := 0, 0\n\ty, z = a/b, f()\n\tfmt.Println(y, z)"},
		{"div-for-init", "", "a, b := 7, 0\n\tfor i, j := a/b, f(); i < j; i++ {\n\t}"},
		{"div-return", "func r(z int) (int, int) {\n\treturn 7 / z, f()\n}\n", "fmt.Println(r(0))"},
		{"op-assign", "", "x /= f() - x\n\tfmt.Println(x)"},
		{"index-call", "", "i := 10\n\tfmt.Println(int(s[i]) + f())"},
		{"index-grow", "", "i := 4\n\tfmt.Println(s[i], grow())"},
		{"index-plus-grow", "", "i := 4\n\tfmt.Println(int(s[i]) + grow())"},
		{"shift-call", "", "n := -1\n\tfmt.Println(1<<n + f())"},
		{"nested-div-first", "", "z := 0\n\tfmt.Println(id(7/z), f())"},
		{"nested-call-first", "", "z := 0\n\tfmt.Println(f(), id(7/z))"},
		{"and-unit", "", "z := 0\n\tfmt.Println(f() > 0 && 7/z > 0, f())"},
		{"for-cond", "", "z := 0\n\tfor i := 0; i < 7/z+f(); i++ {\n\t}"},
		{"for-cond-later", "", "z := 3\n\tfor i := 0; i < 7/z+f(); i++ {\n\t\tz--\n\t}"},
		{"else-if", "", "z := 0\n\tif f() > 100 {\n\t} else if 7/z+f() > 0 {\n\t\tfmt.Println(1)\n\t}"},
		{"case", "", "z := 0\n\tswitch f() {\n\tcase 1:\n\tcase 7/z + f():\n\t\tfmt.Println(1)\n\t}"},
		{"case-list", "", "z := 0\n\tswitch f() {\n\tcase 1, id(7/z) + f():\n\t\tfmt.Println(1)\n\t}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := orderPrelude + tt.decls + "func main() {\n\t" + tt.body + "\n}\n"
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goResult(t, gp)
			if want != nil && want.code != 2 {
				t.Fatalf("with Go, the program exits with %d, not with a panic:\n%s", want.code, want.stderr)
			}
			checkRuns(t, emitter(gp), want)
		})
	}
}

// TestOrderTemps checks where order.go puts temporaries, and where it doesn't: a call gets one
// only next to an operand that is not a constant or a local variable, and that C++ doesn't
// evaluate after it. (TestEmit runs the same cases in testdata/emit/order.gp against Go.)
func TestOrderTemps(t *testing.T) {
	tests := []struct{ body, want string }{
		// No temporaries.
		{"fmt.Println(f())", "gp::fmtshim::Println(f());"},
		{"l := 1\n\tg(l, f())", "g(l, f());"},
		{"g(1, f())", "g(1, f());"},
		{"l := 2\n\ty := f() + l*2\n\tfmt.Println(y)", "int64_t y = f() + l * 2;"},
		{"fmt.Println(g(f(), 1) / 3)", "gp::fmtshim::Println(g(f(), 1) / 3);"},
		{"z := 0\n\tfmt.Println(f() / z)", "gp::fmtshim::Println(gp::div(f(), z));"},
		{"if f() > 0 && f() < 9 {\n\t}", "if (f() > 0 && f() < 9) {"},
		{"for f() < 3 {\n\t}", "while (f() < 3) {"},
		{"a, b := f(), f()\n\tfmt.Println(a, b)", "int64_t a = f();"},
		{"fmt.Println(x)", "gp::fmtshim::Println(x);"},
		{"var b bool\n\tfmt.Println(b, f())", "gp::fmtshim::Println(b, f());"},
		// Temporaries.
		{"fmt.Println(f(), f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp_t0, f()); }"},
		{"g(x, f())", "{ auto gp_t0 = f(); g(x, gp_t0); }"},
		{"g(f(), x)", "{ auto gp_t0 = f(); g(gp_t0, x); }"},
		{"y := x + f()\n\tfmt.Println(y)", "auto gp_t0 = f(); int64_t y = x + gp_t0;"},
		{"x += f()", "{ auto gp_t0 = f(); x = x + gp_t0; }"},
		{"a, z := 7, 0\n\tfmt.Println(a/z + f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp::div(a, z) + gp_t0); }"},
		{"for x < f() {\n\t}", "auto gp_t0 = f(); if (!(x < gp_t0)) break;"},
		{"if f() > 0 && g(x, f()) > 0 {\n\t}", "bool gp_t0 = f() > 0; if (gp_t0) { auto gp_t1 = f(); gp_t0 = g(x, gp_t1) > 0; }"},
		{"s, i, j := \"ab\", 1, 0\n\tfmt.Println(s[i], s[j], s[i] == s[j])", "gp::fmtshim::Println(s[i], s[j], s[i] == s[j]);"},
		{"a, b := 7, 0\n\tfmt.Println(a/b, 1<<b, a%b)", "gp::fmtshim::Println(gp::div(a, b), gp::shl(int64_t(1), b), gp::rem(a, b));"},
		{"fmt.Println(x == 2, f())", "{ auto gp_t0 = x == 2; gp::fmtshim::Println(gp_t0, f()); }"},
		{"println(x == 2, f())", "{ auto gp_t0 = f(); gp::println(x == 2, gp_t0); }"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			dir := t.TempDir()
			src := "package main\n\nimport \"fmt\"\n\nvar x = 1\n\nfunc f() int {\n\tx++\n\tfmt.Print()\n\treturn x\n}\n\n" +
				"func g(a, b int) int {\n\treturn a + b\n}\n\nfunc main() {\n\t" + tt.body + "\n}\n"
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			cpp, err := emitPatterns(t, Config{}, dir, gp)
			if err != nil {
				t.Fatal(err)
			}
			main := cpp[strings.Index(cpp, "void main() {"):]
			if !strings.Contains(main, "\n    "+tt.want+"\n") && !strings.Contains(main, " "+tt.want+"\n") {
				t.Errorf("the C++ has no line\n\t%s\n%s", tt.want, main)
			}
		})
	}
}

// TestOrderLarge checks that ordering a long expression stays fast (plan is O(n^2) in the
// items of a scope): 400 terms, with and without calls, in well under a second.
func TestOrderLarge(t *testing.T) {
	const n = 400
	var plain, calls []string
	for k := range n {
		plain = append(plain, "int(s[i])/i")
		if k%2 == 0 {
			calls = append(calls, "f()")
		} else {
			calls = append(calls, "int(s[i])/i+x")
		}
	}
	dir := t.TempDir()
	src := "package main\n\nvar x = 1\n\nfunc f() int {\n\tx++\n\treturn x\n}\n\nfunc main() {\n\ts, i := \"abc\", 1\n" +
		"\tprintln(" + strings.Join(plain, " + ") + ")\n" +
		"\tprintln(" + strings.Join(calls, ", ") + ")\n" +
		"\tprintln(" + strings.Join(calls, " + ") + ")\n}\n"
	gp := filepath.Join(dir, "main.gp")
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := emitPatterns(t, Config{}, dir, gp); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("loading, checking and emitting took %v, want less than a second", d)
	}
}
