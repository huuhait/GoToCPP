package check

import (
	"strings"
	"testing"
)

// TestGenericCArgs checks the limit of the build that check reports too (genericc.go): an instance
// of a generic type written with a C/C++ type argument, wherever the type is written, once for the
// outermost instance, with the lowering's words; and no error for what the build supports (a Go type
// argument, an alias of an unnamed type, a generic function with a C/C++ type argument) or next to a
// type error of the program.
func TestGenericCArgs(t *testing.T) {
	t.Parallel()
	const decls = "type Box[T any] struct{ v T }\n\ntype Two[K comparable, V any] struct {\n\tk K\n\tv V\n}\n\n" +
		"type Vec[T any] = []T\n\ntype Al[T any] = Box[T]\n\ntype G struct{ c C.int }\n\ntype H C.int\n\n" +
		"func Id[T any](x T) T { return x }\n\n"
	const tail = genericCTail
	for _, c := range []struct{ name, src, want string }{
		{"local variable", "func main() {\n\tvar b Box[C.int]\n\t_ = b\n}", "4:8: " + genericCMsg + "Box[C.int]" + tail},
		{"package variable", "var b Box[C.int]\n\nfunc main() {}", "3:7: " + genericCMsg + "Box[C.int]" + tail},
		{"composite literal", "func main() {\n\t_ = Box[C.int]{}\n}", "4:6: " + genericCMsg + "Box[C.int]" + tail},
		{"two type arguments", "func main() {\n\tvar b Two[int, *C.int]\n\t_ = b\n}", "4:8: " + genericCMsg + "Two[int, *C.int]" + tail},
		{"nested, once", "func main() {\n\tvar b []Box[Box[C.int]]\n\t_ = b\n}", "4:10: " + genericCMsg + "Box[Box[C.int]]" + tail},
		{"parameter", "func f(b Box[C.int]) {}\n\nfunc main() {}", "3:10: " + genericCMsg + "Box[C.int]" + tail},
		{"result", "func f() (b Box[C.int]) { return }\n\nfunc main() {}", "3:13: " + genericCMsg + "Box[C.int]" + tail},
		{"field", "type S struct{ b Box[C.int] }\n\nfunc main() {}", "3:18: " + genericCMsg + "Box[C.int]" + tail},
		{"Go struct with a C field", "func main() {\n\tvar b Box[G]\n\t_ = b\n}", "4:8: " + genericCMsg + "Box[G]" + tail},
		{"named type over a C type", "func main() {\n\tvar b Box[H]\n\t_ = b\n}", "4:8: " + genericCMsg + "Box[H]" + tail},
		{"func type", "func main() {\n\tvar b Box[func(C.int)]\n\t_ = b\n}", "4:8: " + genericCMsg + "Box[func(C.int)]" + tail},
		{"alias of an instance", "func main() {\n\tvar b Al[C.int]\n\t_ = b\n}", "4:8: " + genericCMsg + "Al[C.int]" + tail},
		{"local generic type", "func main() {\n\ttype L[T any] struct{ v T }\n\tvar l L[C.int]\n\t_ = l\n}", "5:8: " + genericCMsg + "L[C.int]" + tail},
		{"two places", "var a Box[C.int]\n\nfunc main() {\n\t_ = Box[C.int]{}\n}",
			"3:7: " + genericCMsg + "Box[C.int]" + tail + "\n./main.gp:6:6: " + genericCMsg + "Box[C.int]" + tail},
		// what the build supports
		{"Go type argument", "func main() {\n\tvar b Box[int]\n\t_ = b\n}", ""},
		{"alias of an unnamed type", "func main() {\n\tvar v Vec[C.int]\n\t_ = v\n}", ""},
		{"generic function", "func main() {\n\tvar x C.int\n\t_ = Id[C.int](x)\n}", ""},
		// next to a type error: that error only
		{"undefined type argument", "func main() {\n\tvar b Box[Nosuch]\n\t_ = b\n}", "4:12: undefined: Nosuch"},
		{"type error elsewhere", "func main() {\n\tvar b Box[C.int]\n\t_ = b\n\tvar n int = \"s\"\n\t_ = n\n}",
			"6:14: cannot use \"s\" (untyped string constant) as int value in variable declaration"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := checkErrors(t, LastPhase, "package main\n\n"+c.src+"\n\n"+decls)
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

// TestGenericInstancesWithCArgs checks the uses of generic functions and types whose instances would
// make an instance of a generic type with a C/C++ type argument (genericcinst.go): reported at the use,
// with the generic type and what makes it, for a type argument written or deduced, through other
// generic functions (also a recursive one, and two that call each other), a type declared in the
// function, the signature, an instance written in a generic declaration (only once it is used); not
// for a Go type argument, a generic function that makes no generic type, a key that the argument's map
// type gives (Put), a constraint interface of a type parameter, a local alias, a generic declaration
// that is never instantiated.
func TestGenericInstancesWithCArgs(t *testing.T) {
	t.Parallel()
	const decls = "type Box[T any] struct{ v T }\n\n" +
		"func Wrap[T any](v T) int {\n\tb := Box[T]{v}\n\t_ = b\n\treturn 1\n}\n\n" +
		"func Outer[T any](v T) int { return Wrap(v) }\n\n" +
		"func Rec[T any](v T, n int) int {\n\tif n == 0 {\n\t\treturn Wrap(v)\n\t}\n\treturn Rec(v, n-1)\n}\n\n" +
		"func Loc[T any](v T) int {\n\ttype pair struct{ n int }\n\treturn pair{1}.n\n}\n\n" +
		"func Sig[T any](v T) Box[T] { return Box[T]{v} }\n\n" +
		"func Plain[T any](v T) T { return v }\n\n" +
		"func Put[M ~map[K]V, K comparable, V any](m M, k K, v V) Box[K] { return Box[K]{k} }\n\n" +
		"type SliceOf[T any] interface{ ~[]T }\n\n" +
		"func Count2[S SliceOf[T], T any](xs S) int { return len(xs) }\n\n" +
		"func Alias[T any](v T) int {\n\ttype P = struct{ n int }\n\tf := func() int {\n\t\ttype Q = int\n\t\tvar q Q = 2\n\t\treturn q\n\t}\n\treturn P{1}.n + f()\n}\n\n" +
		"func Never[T any]() int {\n\tvar b Box[C.int]\n\t_ = b\n\treturn 0\n}\n\n" +
		"type W[T any] struct{ b Box[C.int] }\n\n" +
		"func F[T, U any](t T, u U, n int) int {\n\tif n > 0 {\n\t\treturn G(t, u, n-1)\n\t}\n\tvar b Box[U]\n\t_ = b\n\treturn 1\n}\n\n" +
		"func G[T, U any](t T, u U, n int) int { return F(t, u, n) }\n\n" +
		"type G2 struct{ c C.int }\n\n"
	msg := func(typ, fn, desc string) string { return genericInstMsg(fn, genericReason{typ, desc}) }
	for _, c := range []struct{ name, src, want string }{
		{"deduced", "func main() {\n\tvar x C.int\n\t_ = Wrap(x)\n}", "5:6: " + msg("Box", "Wrap", "makes Box[T]")},
		{"written", "func main() {\n\tvar x C.int\n\t_ = Wrap[C.int](x)\n}", "5:6: " + msg("Box", "Wrap", "makes Box[T]")},
		{"Go struct with a C field", "func main() {\n\t_ = Wrap(G2{})\n}", "4:6: " + msg("Box", "Wrap", "makes Box[T]")},
		{"through another function", "func main() {\n\tvar x C.int\n\t_ = Outer(x)\n}", "5:6: " + msg("Box", "Outer", "calls Wrap, which makes Box[T]")},
		{"recursive", "func main() {\n\tvar x C.int\n\t_ = Rec(x, 2)\n}", "5:6: " + msg("Box", "Rec", "calls Wrap, which makes Box[T]")},
		{"mutually recursive", "func main() {\n\tvar x C.int\n\t_ = F[int, C.int](1, x, 1)\n\t_ = G[int, C.int](1, x, 1)\n}",
			"5:6: " + msg("Box", "F", "makes Box[U]") + "\n./main.gp:6:6: " + msg("Box", "G", "calls F, which makes Box[U]")},
		{"mutually recursive, Go argument for U", "func main() {\n\tvar x C.int\n\t_ = F[C.int, int](x, 1, 1)\n\t_ = G[C.int, int](x, 1, 1)\n}", ""},
		// (deduced, U is a stand-in too: go/types records no instance, and the build refuses it as well)
		{"mutually recursive, deduced", "func main() {\n\tvar x C.int\n\t_ = F(x, 1, 1)\n}", "5:6: " + msg("Box", "F", "makes Box[U]")},
		{"local type", "func main() {\n\tvar x C.int\n\t_ = Loc(x)\n}", "5:6: " + msg("pair", "Loc", "declares pair")},
		{"signature", "func main() {\n\tvar x C.int\n\t_ = Sig(x)\n}", "5:6: " + msg("Box", "Sig", "makes Box[T]")},
		{"func value", "func main() {\n\tf := Wrap[C.int]\n\t_ = f\n}", "4:7: " + msg("Box", "Wrap", "makes Box[T]")},
		{"Go type argument", "func main() {\n\t_ = Wrap(1) + Outer(\"s\") + Loc(2.5) + Rec(1, 2)\n}", ""},
		{"no generic type", "func main() {\n\tvar x C.int\n\t_ = Plain(x)\n}", ""},
		{"key from the map", "func main() {\n\tvar x C.int\n\tm := map[int]C.int{}\n\t_ = Put(m, 1, x)\n}", ""},
		{"C key of the map", "func main() {\n\tvar x C.int\n\tm := map[C.int]int{}\n\t_ = Put(m, x, 1)\n}", "6:6: " + msg("Box", "Put", "makes Box[K]")},
		{"constraint interface", "func main() {\n\tvar xs []C.int\n\t_ = Count2(xs)\n}", ""},
		{"local aliases", "func main() {\n\tvar x C.int\n\t_ = Alias(x)\n}", ""},
		{"never instantiated", "func main() {}", ""},
		{"instantiated with a Go type", "func main() {\n\t_ = Never[int]()\n}", "4:6: " + msg("Box", "Never", "makes Box[C.int]")},
		{"generic type never instantiated", "func main() {}", ""},
		{"generic type instantiated with a Go type", "func main() {\n\tvar w W[int]\n\t_ = w\n}",
			"4:8: " + genericTypeMsg("W[int]", genericReason{"Box", "has Box[C.int]"})},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := checkErrors(t, LastPhase, "package main\n\n"+c.src+"\n\n"+decls)
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
