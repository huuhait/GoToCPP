// phase: 08

// A callee that borrows a pointer parameter (only reads it) but is not pure (it drops the package
// variable that the caller passed): the argument must be lent from a copy. This holds for a
// variadic callee too, with or without trailing arguments, called as a function, as a method, with
// `xs...`, through an interface and through a method expression (ADR-086).
package main

import "fmt"

type T struct {
	n    int
	name string
}

var g *T
var gi Bumper

type Bumper interface{ Bump(o *T, xs ...int) int }

func use(t *T, xs ...int) int {
	g = nil
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return t.n + len(t.name) + len(xs) + len(junk)
}

func (t *T) m(o *T, xs ...int) int {
	g = nil
	gi = nil
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return t.n + o.n + len(o.name) + len(xs) + len(junk)
}

func (t *T) Bump(o *T, xs ...int) int { return t.m(o, xs...) }

func (t *T) viaSlice(xs ...int) int {
	g = nil
	return t.n + len(t.name) + len(xs)
}

func fresh(n int) *T { return &T{n: n, name: "ccc"} }

func main() {
	g = fresh(4)
	fmt.Println(use(g, 1, 2))
	g = fresh(5)
	fmt.Println(use(g))
	g = fresh(6)
	xs := []int{1, 2, 3}
	fmt.Println(use(g, xs...))

	g = fresh(7)
	fmt.Println(fresh(1).m(g, 1))
	g = fresh(8)
	fmt.Println(fresh(1).m(g))
	g = fresh(9)
	fmt.Println(fresh(1).m(g, xs...))

	// the receiver itself is the package variable
	g = fresh(10)
	fmt.Println(g.viaSlice(1, 2))
	g = fresh(11)
	fmt.Println(g.viaSlice())
	g = fresh(12)
	fmt.Println(g.viaSlice(xs...))

	// through an interface: the interface variable holds the receiver
	gi = fresh(20)
	g = fresh(13)
	fmt.Println(gi.Bump(g, 1, 2))
	gi = fresh(21)
	g = fresh(14)
	fmt.Println(gi.Bump(g))

	// a method expression, called directly
	g = fresh(15)
	fmt.Println((*T).m(fresh(1), g, 1, 2))
	g = fresh(16)
	fmt.Println((*T).m(fresh(1), g))
	g = fresh(17)
	fmt.Println((*T).m(fresh(1), g, xs...))

	// a method value
	g = fresh(18)
	f := fresh(1).m
	fmt.Println(f(g, 1))
}

// Output:
// 19
// 18
// 22
// 22
// 22
// 26
// 15
// 14
// 18
// 48
// 48
// 31
// 30
// 34
// 33
