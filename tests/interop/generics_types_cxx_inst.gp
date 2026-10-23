// phase: 07

// A generic function or type whose instance makes an instance of a generic type with a C++ type argument: in its body (Box[T]), in
// its result (Box[V]), in a type declared in it (which takes the function's type parameters), through another generic function, also
// two that call each other, with the type argument written (Wrap[std.string]) or deduced by C++; and a generic declaration that writes
// Box[std.string] itself, which only an instance meets (Never[int](), W[int]). check, vet and the build report each use, and say what
// the function makes; a Go type argument works.
package main

import (
	"fmt"

	"<string>"
)

type Box[T any] struct{ v T }

func Wrap[T any](v T) int {
	b := Box[T]{v}
	_ = b
	return 1
}

func Outer[T any](v T) int { return Wrap(v) }

func Mk[K comparable, V any](m map[K]V) Box[V] {
	var z Box[V]
	return z
}

func Loc[T any](v T) int {
	type pair struct{ n int }
	return pair{1}.n
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

func Never[T any]() int {
	var b Box[std.string]
	_ = b
	return 0
}

type W[T any] struct{ b Box[std.string] }

func main() {
	fmt.Println(Wrap(std.string("y")))                     // ERROR "an instance of the generic type Box with a C/C\+\+ type argument of a generic function is not supported yet \(Wrap makes Box\[T\]\): the type arguments of a generic type are Go types"
	fmt.Println(Wrap[std.string](std.string("y")))         // ERROR "\(Wrap makes Box\[T\]\)"
	fmt.Println(Outer(std.string("x")))                    // ERROR "\(Outer calls Wrap, which makes Box\[T\]\)"
	_ = Mk(map[int]std.string{})                           // ERROR "\(Mk makes Box\[V\]\)"
	fmt.Println(Loc(std.string("z")))                      // ERROR "an instance of the generic type pair with a C/C\+\+ type argument of a generic function is not supported yet \(Loc declares pair\)"
	fmt.Println(G[int, std.string](1, std.string("u"), 1)) // ERROR "\(G calls F, which makes Box\[U\]\)"
	fmt.Println(Never[int]())                              // ERROR "\(Never makes Box\[C.std.string\]\)"
	var w W[int]                                           // ERROR "an instance of the generic type Box with a C/C\+\+ type argument is not supported yet \(W\[int\] has Box\[C.std.string\]\)"
	_ = w
	fmt.Println(Wrap(1), Outer("s"), Loc(2.5), Mk(map[int]string{}), F[std.string, int](std.string("t"), 1, 1))
}
