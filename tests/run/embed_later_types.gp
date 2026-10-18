// phase: 05
// lsan: cycle

// Embedding pointers to types declared later, and mutually recursive embedded pointers (a reference
// cycle that reference counting never frees, D1: lsan: cycle): the C++
// structs are defined in dependency order, and the promoted names are found through the cycle.
package main

import "fmt"

type A struct {
	*B
	a int
}

type B struct {
	*A
	b int
}

func (a *A) HereA() string { return fmt.Sprint("A", a.a) }
func (b *B) HereB() string { return fmt.Sprint("B", b.b) }

type First struct {
	Second
	f int
}

type Second struct {
	Third
	s int
}

type Third struct{ t int }

func (t Third) T() int { return t.t }

func main() {
	a := &A{a: 1}
	a.B = &B{A: a, b: 2}
	fmt.Println(a.HereA(), a.HereB(), a.B.HereA(), a.B.A.HereB(), a.b, a.B.a, a.B.A.B.b)
	f := First{Second{Third{3}, 2}, 1}
	fmt.Println(f.T(), f.t, f.s, f.f)
}

// Output:
// A1 B2 A1 B2 2 1 2
// 3 3 2 1
