// phase: 07

// A struct field of func type with a C++ parameter or result, in a generic type (f func(std.string)
// T of H[int]: go/types makes a new field with T replaced, whose C++ type comes from the generic
// declaration's type expression) and in a plain one, called directly (h.f(x), p.f(x), o.h.f(x),
// fs[0](x), m[k](x)), from a method and from a generic function; a local of a generic function whose
// written type has a C++ part and a type parameter (var cb func(std.string) T), and a parameter and an
// unnamed result of such a type, and a struct type declared in a generic function with such a field
// (the function's type parameters are the type's: localtypes.go).
package main

import (
	"fmt"

	"<string>"
)

type H[T any] struct {
	f    func(std.string) T
	g    func(int) std.string
	name std.string
	n    T
}

func (h H[T]) Call(s std.string) T { return h.f(s) }

func (h *H[T]) Set(x std.string) { h.name = h.g(int(x.size())) }

type NH struct {
	f  func(std.string) int
	fs []func(std.string) int
	m  map[string]func(std.string) bool
}

type Outer struct {
	h H[bool]
	k int
}

func Use[T any](h H[T], s std.string) T { return h.f(s) }

func Local[T any](v T) T {
	var cb func(std.string) T
	cb = func(s std.string) T { return v }
	call := func(s std.string) T { return cb(s) }
	return call(std.string("q"))
}

func Apply[T any](f func(std.string) T, s std.string) T { return f(s) }

func Ret[T any](v T) func(std.string) T { return func(std.string) T { return v } }

func LocalType[T any](v T) T {
	type L struct {
		f func(std.string) T
	}
	l := L{f: func(std.string) T { return v }}
	return l.f(std.string("x"))
}

func main() {
	h := H[int]{
		f: func(s std.string) int { return int(s.size()) * 10 },
		g: func(n int) std.string { return std.string(fmt.Sprint("n", n)) },
	}
	fmt.Println(h.f(std.string("x")), h.Call(std.string("xy")), Use[int](h, std.string("abc")))
	p := &h
	fmt.Println(p.f(std.string("abcd")))
	p.Set(std.string("hello"))
	fmt.Println(h.name, h.g(3))

	var z H[string]
	fmt.Println(z.f == nil, z.n == "")
	z.f = func(s std.string) string { return fmt.Sprint("len ", int(s.size())) }
	fmt.Println(z.Call(std.string("zz")))

	o := Outer{h: H[bool]{f: func(s std.string) bool { return bool(s.empty()) }}}
	fmt.Println(o.h.f(std.string("")), o.h.f(std.string("a")), o.k)

	nh := NH{f: func(s std.string) int { return int(s.size()) + 1 }}
	nh.fs = append(nh.fs, nh.f)
	nh.m = map[string]func(std.string) bool{"e": func(s std.string) bool { return bool(s.empty()) }}
	fmt.Println(nh.f(std.string("xy")), nh.fs[0](std.string("abc")), nh.m["e"](std.string("")))

	fmt.Println(Local(5), Local("s"))
	fmt.Println(Apply[int](func(s std.string) int { return int(s.size()) }, std.string("abc")))
	fmt.Println(Ret(4)(std.string("x")), Ret[string]("s")(std.string("y")))
	fmt.Println(LocalType(7), LocalType("t"))
}

// Output:
// 10 20 30
// 40
// n5 n3
// true true
// len 2
// true false 0
// 3 4 true
// 5 s
// 3
// 4 s
// 7 t
