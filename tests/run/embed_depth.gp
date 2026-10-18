// phase: 05

// Promoted fields and methods through several levels, Go's depth rule (the shallowest wins, an
// outer name shadows a deeper one), the full path written out, and the method of the outer type
// that hides the promoted one.
package main

import "fmt"

type A struct{ X, Y int }

func (a A) Sum() int     { return a.X + a.Y }
func (a *A) Inc()        { a.X++; a.Y++ }
func (a A) Name() string { return "A" }
func (a A) OnlyA() string { return "only A" }

type B struct {
	A
	Z int
}

func (b B) Name() string { return "B" }

type Mid struct {
	B
	X string // hides A.X, two levels down
}

type D struct {
	*Mid
	W int
}

func main() {
	c := Mid{B: B{A: A{1, 2}, Z: 3}, X: "str"}
	fmt.Println(c.X, c.B.X, c.A.X, c.B.A.X, c.Y, c.Z, c.Sum(), c.Name(), c.A.Name(), c.OnlyA())
	c.Inc()
	fmt.Println(c.B.A, c.Sum())
	d := D{&c, 9}
	d.Inc()
	d.Z = 77
	fmt.Println(d.X, d.A.X, d.Sum(), c.Z, d.Name(), d.Mid.B.Name(), d.W)
	d.X = "other"
	d.A.X = 40
	fmt.Println(c.X, c.A.X, d.Y)
	pa := &d.A
	pa.Inc()
	fmt.Println(c.A, c.B.A.X)
	dd := &d
	dd.Y += 100
	fmt.Println(dd.Sum(), dd.Mid.B.A.Y)
}

// Output:
// str 1 1 1 2 3 3 B A only A
// {2 3} 5
// str 3 7 77 B B 9
// other 40 4
// {41 5} 41
// 146 105
