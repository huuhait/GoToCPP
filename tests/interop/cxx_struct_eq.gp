// phase: 05

// Go's == of gocpp structs and arrays with C/C++ parts compares the C/C++ parts with C++'s ==: directly
// (==, !=), in a switch on such a value, through interfaces (any == any), as the keys of a map[any]T, and
// for a named array type over such a struct (type A [2]S), also as a map key.
package main

import (
	"fmt"

	"./map_cxx_key_class.hpp"
	"<string>"
)

type S struct {
	name std.string
	n    int
}

// A named array type over such a struct: its size is the C++ compiler's (no layout assert, named.go).
type A [2]S

type P struct {
	at mk.Pt
	_  int
	ok bool
}

func main() {
	x, y, z := S{name: std.string("x")}, S{name: std.string("x")}, S{name: std.string("z"), n: 1}
	fmt.Println(x == y, x != y, x == z, x != z)

	a, b := [2]std.string{"a", "b"}, [2]std.string{"a", "b"}
	fmt.Println(a == b, a != [2]std.string{"a", "c"})

	p, q := P{at: mk.Pt{x: 1, y: 2}, ok: true}, P{at: mk.Pt{x: 1, y: 2}, ok: true}
	fmt.Println(p == q, p == P{})

	switch z {
	case x:
		fmt.Println("x")
	case S{name: std.string("z"), n: 1}:
		fmt.Println("z")
	default:
		fmt.Println("none")
	}

	var i, j any = x, y
	fmt.Println(i == j, i == any(z))
	m := map[any]int{}
	m[x] = 1
	m[y] = 2
	m[z] = 3
	fmt.Println(len(m), m[S{name: std.string("x")}], m[z])

	var na, nb A
	na[1].n = 4
	nb[1].n = 4
	fmt.Println(na == nb, na != A{}, len(na))
	byA := map[A]int{na: 1}
	byA[nb]++
	byA[A{}] = 7
	fmt.Println(len(byA), byA[na], byA[A{}])
}

// Output:
// true false false true
// true true
// true false
// z
// true false
// 2 2 3
// true true 2
// 2 2 7
