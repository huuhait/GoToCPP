package server

import (
	"context"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/lsp/cache"
)

// TestDiagnosticsGenericCxxLimits: the editor shows the limits of C/C++ type arguments in generic code
// that gocpp check and the build report (internal/check: genericc.go, genericcinst.go, pkgcvars.go), at
// the same places with the same words: a written instance of a generic type with a C++ type argument,
// a use of a generic function whose instance would make one, and a package variable from a generic
// call whose type argument C++ deduces.
func TestDiagnosticsGenericCxxLimits(t *testing.T) {
	src := "package main\n\nimport \"<string>\"\n\ntype Box[T any] struct{ v T }\n\n" +
		"func Wrap[T any](v T) int {\n\t_ = Box[T]{v}\n\treturn 1\n}\n\n" +
		"func Seq[T any](xs []T) func(func(T) bool) { return nil }\n\n" +
		"var words []std.string\n\nvar it = Seq(words)\n\n" +
		"func main() {\n\tvar b Box[std.string]\n\t_ = b\n\t_ = Wrap(words[0])\n}\n"
	want := "15:4-15:6 error: package variables of C/C++ values without a type are not supported yet: write the type of it (var it T = ...) or the type arguments of Seq (Seq[std.string](...))\n" +
		"18:7-18:10 error: an instance of a generic type with a C/C++ type argument (Box[C.std.string]) is not supported yet: the type arguments of a generic type are Go types\n" +
		"20:5-20:9 error: an instance of the generic type Box with a C/C++ type argument of a generic function is not supported yet (Wrap makes Box[T]): the type arguments of a generic type are Go types\n"
	c := startDiag(t, fakeStdProbe(testConfig()), map[string]string{"gocpp.mod": "module example.com/m\n"}, nil)
	c.open("main.gp", src)
	waitForLimits(t, c, want)
}

// TestDiagnosticsGenericCxxStdIterators: the generic functions of the standard packages come from export
// data in the editor (no declaration to summarize): their signatures say what their instances make, so
// slices.Values, slices.All and maps.Values with C++ elements get the error of gocpp check too, and a Go
// element type gets none.
func TestDiagnosticsGenericCxxStdIterators(t *testing.T) {
	src := "package main\n\nimport (\n\t\"fmt\"\n\t\"maps\"\n\t\"slices\"\n\n\t\"<string>\"\n)\n\n" +
		"func main() {\n\twords := []std.string{}\n\tm := map[int]std.string{}\n" +
		"\tfor w := range slices.Values(words) {\n\t\tfmt.Println(int(w.size()))\n\t}\n" +
		"\tfor _, w := range slices.All(words) {\n\t\tfmt.Println(int(w.size()))\n\t}\n" +
		"\tfor w := range maps.Values(m) {\n\t\tfmt.Println(int(w.size()))\n\t}\n" +
		"\tfor w := range slices.Values([]int{1}) {\n\t\tfmt.Println(w)\n\t}\n}\n"
	want := "13:16-13:22 error: an instance of the generic type Seq with a C/C++ type argument of a generic function is not supported yet (Values makes iter.Seq[E]): the type arguments of a generic type are Go types\n" +
		"16:19-16:25 error: an instance of the generic type Seq2 with a C/C++ type argument of a generic function is not supported yet (All makes iter.Seq2[int, E]): the type arguments of a generic type are Go types\n" +
		"19:16-19:20 error: an instance of the generic type Seq with a C/C++ type argument of a generic function is not supported yet (Values makes iter.Seq[V]): the type arguments of a generic type are Go types\n"
	c := startDiag(t, fakeStdProbe(testConfig()), map[string]string{"gocpp.mod": "module example.com/m\n"}, nil)
	c.open("main.gp", src)
	waitForLimits(t, c, want)
}

// fakeStdProbe gives cfg a C++ compiler and a namespace probe that knows only std.
func fakeStdProbe(cfg cache.Config) cache.Config {
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = func(_ context.Context, _ build.CXX, _ string, _, _, cands []string) (interop.Result, error) {
		r := interop.Result{Kinds: make(map[string]interop.Kind)}
		for _, c := range cands {
			r.Kinds[c] = interop.NotNamespace
			if c == "std" {
				r.Kinds[c] = interop.Namespace
			}
		}
		return r, nil
	}
	return cfg
}

// waitForLimits waits until the diagnostics of main.gp are want; other diagnostics with a limit of
// C/C++ type arguments fail the test.
func waitForLimits(t *testing.T, c *dclient, want string) {
	t.Helper()
	for {
		got := format(c.next("main.gp")) // fails after a minute
		if got == want {
			return
		}
		if strings.Contains(got, "not supported yet") {
			t.Fatalf("diagnostics:\n%s\nwant:\n%s", got, want)
		}
	}
}
