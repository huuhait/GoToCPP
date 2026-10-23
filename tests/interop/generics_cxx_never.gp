// phase: 07

// Generic code with C++ types that gocpp builds, which check must not refuse: a generic declaration that writes Box[std.string] but
// is never instantiated (a function, a generic type), a constraint interface of a type parameter (SliceOf[T] is no instance), local
// aliases in a generic function and in a func literal in one (no type of the instance), and two generic functions that call each other
// with a Go type for the type parameter of their Box.
package main

import (
	"fmt"

	"<string>"
)

type Box[T any] struct{ v T }

func Never[T any]() int {
	var b Box[std.string]
	_ = b
	return 0
}

type W[T any] struct{ b Box[std.string] }

type SliceOf[T any] interface{ ~[]T }

func Count2[S SliceOf[T], T any](xs S) int {
	n := 0
	for range xs {
		n++
	}
	return n
}

func Alias[T any](x T) int {
	type P = struct{ n int }
	var p P
	p.n = 1
	f := func() int {
		type Q = int
		var q Q = 2
		return q
	}
	return p.n + f()
}

func F[T, U any](t T, u U, n int) int {
	if n > 0 {
		return G(t, u, n-1)
	}
	var b Box[U]
	b.v = u
	return 1
}

func G[T, U any](t T, u U, n int) int { return F(t, u, n) }

func main() {
	words := []std.string{"a", "bcd"}
	fmt.Println(Count2(words))
	fmt.Println(Alias(std.string("x")))
	fmt.Println(F[std.string, int](std.string("x"), 1, 1))
}

// Output:
// 2
// 3
// 1
