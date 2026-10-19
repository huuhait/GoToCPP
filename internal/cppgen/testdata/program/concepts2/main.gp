package main

import (
	"fmt"

	"example.com/concepts2/lib"
)

type x struct{ n int }
type a0 int

type G interface{ Get(x, a0) int }

type impl struct{}

func (impl) Get(p x, q a0) int { return p.n + int(q) }

// Names that the text of a concept uses.
func F[T G](v T) int                              { return v.Get(x{2}, 1) }
func A[std interface{ Hello() string }](v std) string { return v.Hello() }
func B[gp ~int | ~string](v gp) gp                { return v }
func Cc[x interface{ Hello() string }](v x) string { return v.Hello() }
func D[a0 lib.Number](v a0) a0                    { return v + v }

func V[T lib.Visitor](v T) int { return v.Visit(1, "a", "b") }

func H[T lib.Greeter](v T) string { return v.Hello() }

func Ad[T lib.Adder](v T) { v.Add(5) }

func main() {
	fmt.Println(F(impl{}), A(lib.S{}), B(3), B("s"), Cc(lib.Número(1)), D(2.5), D(lib.Count(0).Get()))
	fmt.Println(V(lib.Visit{}), H(lib.S{}), H(lib.Número(2)), lib.Twice(4))
	var c lib.Count
	Ad(&c)
	fmt.Println(c.Get(), c.Sum(1, 2))
}
