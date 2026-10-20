// phase: 08
// ExitCode: 2
// Stderr: panic: boom-d

// Functions that call recover() and are deferred with a lent pointer: the deferred call keeps its own copy, and the panicking function drops the package variable (ADR-086).

package main

import "fmt"

type T struct {
	n    int
	name string
}

var g *T

func junk() int {
	t := 0
	for i := 0; i < 100; i++ {
		x := &T{n: i, name: "junkjunkjunk"}
		t += x.n
	}
	return t
}

// guard is a deferred function that calls recover: an aware function.
func guard(t *T, tag string) {
	r := recover()
	junk()
	fmt.Println(tag, r, t.n, t.name)
}

func (t *T) guardM(tag string) {
	r := recover()
	junk()
	fmt.Println(tag, r, t.n, t.name)
}

func a() {
	g = &T{n: 1, name: "one-long-name"}
	defer guard(g, "a")
	g = nil
	junk()
	panic("boom-a")
}

func b() {
	g = &T{n: 2, name: "two-long-name"}
	defer g.guardM("b")
	g = nil
	junk()
	panic("boom-b")
}

func c() {
	g = &T{n: 3, name: "three-long-name"}
	defer guard(g, "c")
	g = nil
	junk()
}

func d() {
	t := &T{n: 4, name: "four-long-name"}
	f := func() { guard(t, "d") }
	t = &T{n: 44, name: "four-four"}
	defer f()
	panic("boom-d")
}

func main() {
	a()
	b()
	c()
	d()
}

// Output:
// a boom-a 1 one-long-name
// b boom-b 2 two-long-name
// c <nil> 3 three-long-name
// d <nil> 44 four-four
