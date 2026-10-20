// phase: 08

// A pure callee (it only reads) borrows its pointer arguments, but another argument of the same
// call runs code that replaces the object that the first argument pointed to (the old one is
// freed when the last reference goes). Go reads either value, the two objects are equal, so the
// output does not depend on the order of evaluation; a crash or an ASan report is the failure
// (ADR-086).
package main

import "fmt"

type T struct {
	n    int
	name string
}

var g *T
var h struct{ p *T }
var gi Reader

type Reader interface{ Read(k int) int }

func clone() int {
	g = &T{n: g.n, name: g.name}
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return len(junk)
}

func cloneH() int {
	h.p = &T{n: h.p.n, name: h.p.name}
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return len(junk)
}

func cloneI() int {
	t := gi.(*T)
	gi = &T{n: t.n, name: t.name}
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return len(junk)
}

func read(t *T, k int) int { return t.n + len(t.name) + k }

func (t *T) Read(k int) int { return t.n + len(t.name) + k }

func (t *T) read2(o *T, k int) int { return t.n + o.n + len(t.name) + len(o.name) + k }

func main() {
	g = &T{n: 4, name: "ccc"}
	fmt.Println(read(g, clone()))
	g = &T{n: 5, name: "ccc"}
	fmt.Println(g.Read(clone()))
	g = &T{n: 6, name: "ccc"}
	fmt.Println(g.read2(g, clone()))
	h.p = &T{n: 7, name: "ccc"}
	fmt.Println(read(h.p, cloneH()))
	h.p = &T{n: 8, name: "ccc"}
	fmt.Println(h.p.Read(cloneH()))
	gi = &T{n: 9, name: "ccc"}
	fmt.Println(gi.Read(cloneI()))
	gi = &T{n: 10, name: "ccc"}
	fmt.Println(read(gi.(*T), cloneI()))
}

// Output:
// 17
// 18
// 28
// 20
// 21
// 22
// 23
