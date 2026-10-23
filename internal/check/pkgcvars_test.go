package check

import (
	"strings"
	"testing"
)

// TestPkgCVars checks the package variables declared without a type from a call of a generic function
// with a type argument that C++ deduces, which check reports as the build does (pkgcvars.go), and
// those it does not report: a call of an instance whose type arguments are all known (also from a
// written slice type), a written type, a Go value, a blank variable, a local, and another C/C++ value
// (refused by the build alone).
func TestPkgCVars(t *testing.T) {
	t.Parallel()
	const decls = "func Seq[T any](xs []T) func(yield func(T) bool) { return nil }\n\n" +
		"func Last[S ~[]T, T any](xs S) T { return xs[0] }\n\n" +
		"func Two[T any](x T) (T, T) { return x, x }\n\n" +
		"func cv() C.int { return 1 }\n\n" +
		"var xs []C.int\n"
	for _, c := range []struct{ name, src, want string }{
		{"C value (the build's error alone)", "var a = cv()", ""},
		{"deduced", "var a = Seq(xs)", "3:5: " + PkgCVarMessage("a", "Seq", []string{"C.int"})},
		{"two variables", "var a, b = Two(cv())", "3:5: " + PkgCVarMessage("a", "Two", []string{"T"}) +
			"\n./main.gp:3:8: " + PkgCVarMessage("b", "Two", []string{"T"})},
		{"written type arguments", "var a = Seq[C.int](xs)", ""},
		{"written slice type", "var a = Last[[]C.int](xs)", ""},
		{"two variables, written", "var a, b = Two[C.int](cv())", ""},
		{"written type", "var a C.int = cv()", ""},
		{"Go value", "var a = len(xs)", ""},
		{"blank", "var _ = cv()", ""},
		{"local", "func f() {\n\ta := cv()\n\t_ = a\n}", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := checkErrors(t, LastPhase, "package main\n\n"+c.src+"\n\nfunc main() {}\n\n"+decls)
			got = strings.TrimSpace(strings.TrimPrefix(got, "# example.com/m\n"))
			want := ""
			if c.want != "" {
				want = "./main.gp:" + c.want
			}
			if got != want {
				t.Errorf("errors:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
