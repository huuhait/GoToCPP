// phase: 05

// Promoted methods of every shape: variadic, several results, named results, struct and slice
// parameters, func parameters, a method that changes its receiver and one that returns it.
package main

import "fmt"

type Pair struct{ A, B int }

type Calc struct{ acc int }

func (c *Calc) Add(xs ...int) int {
	for _, x := range xs {
		c.acc += x
	}
	return c.acc
}
func (c Calc) DivMod(a, b int) (q, r int) { return a / b, a % b }
func (c Calc) Swap(p Pair) Pair            { return Pair{p.B, p.A} }
func (c Calc) Apply(f func(int) int, xs []int) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = f(x) + c.acc
	}
	return out
}
func (c *Calc) Self() *Calc            { return c }
func (c Calc) Copy() Calc              { return c }
func (c Calc) Named() (n int, s string) { n = c.acc; s = "named"; return }
func (c Calc) None()                   {}

type Mid struct{ Calc }
type Top struct {
	*Mid
	label string
}

func main() {
	t := Top{&Mid{}, "t"}
	fmt.Println(t.Add(1, 2, 3), t.Add(), t.Add([]int{4, 5}...))
	q, r := t.DivMod(17, 5)
	fmt.Println(q, r, t.Swap(Pair{1, 2}), t.Apply(func(x int) int { return x * 2 }, []int{1, 2}))
	fmt.Println(t.Self() == &t.Mid.Calc, t.Copy().acc, t.Mid.Copy() == t.Calc)
	n, s := t.Named()
	t.None()
	fmt.Println(n, s)
	fmt.Println(fmt.Sprint(t.DivMod(9, 4)))
	var m Mid
	m.Add(10)
	fmt.Println(m.Self().acc, m.Copy().Swap(Pair{3, 4}))
}

// Output:
// 6 6 15
// 3 2 {2 1} [17 19]
// true 15 true
// 15 named
// 2 1
// 10 {4 3}
