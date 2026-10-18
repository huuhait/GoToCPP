// phase: 05

// Promoted methods of every shape through an itab: variadic, several results, a struct and a
// slice and a func as parameters, results that are structs, methods of an embedded named number,
// slice and func type, and a receiver larger than 128 KiB.
package main

import "fmt"

type Pair struct{ A, B int }

type Calc interface {
	Add(xs ...int) int
	DivMod(a, b int) (int, int)
	Swap(p Pair) Pair
	Apply(f func(int) int, xs []int) []int
	Self() Calc
}

type engine struct{ acc int }

func (e *engine) Add(xs ...int) int {
	for _, x := range xs {
		e.acc += x
	}
	return e.acc
}
func (e *engine) DivMod(a, b int) (int, int) { return a / b, a % b }
func (e *engine) Swap(p Pair) Pair           { return Pair{p.B, p.A} }
func (e *engine) Apply(f func(int) int, xs []int) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = f(x) + e.acc
	}
	return out
}
func (e *engine) Self() Calc { return e }

type Mid struct{ *engine }
type Top struct {
	Mid
	name string
}

type Celsius float64

func (c Celsius) F() float64  { return float64(c)*9/5 + 32 }
func (c *Celsius) Set(v float64) { *c = Celsius(v) }

type IDs []int

func (s IDs) Len() int { return len(s) }

type Op func(int) int

func (f Op) Twice(x int) int { return f(f(x)) }

type Temp interface {
	F() float64
	Len() int
	Twice(int) int
}

type Sensor struct {
	Celsius
	IDs
	Op
}

type Setter interface{ Set(float64) }

type Big struct{ data [40000]int }

func (b *Big) Fill(v int) { b.data[len(b.data)-1] = v }
func (b Big) Last() int   { return b.data[len(b.data)-1] }

type Holder struct {
	Big
	n int
}

type Filler interface{ Fill(int) }
type Lasting interface{ Last() int }

func main() {
	var c Calc = Top{Mid{&engine{}}, "t"}
	fmt.Println(c.Add(1, 2, 3), c.Add(), c.Add([]int{4, 5}...))
	q, r := c.DivMod(17, 5)
	fmt.Println(q, r, c.Swap(Pair{1, 2}), c.Apply(func(x int) int { return x * 2 }, []int{1, 2}))
	fmt.Println(c.Self().Add(1))
	s := Sensor{100, IDs{1, 2, 3}, func(x int) int { return x * 3 }}
	var t Temp = s
	fmt.Println(t.F(), t.Len(), t.Twice(2))
	var st Setter = &s
	st.Set(37)
	fmt.Println(s.Celsius, s.F())
	h := &Holder{}
	var f Filler = h
	f.Fill(9)
	var l Lasting = *h
	h.Fill(10)
	var lp Lasting = h
	fmt.Println(l.Last(), lp.Last(), h.Last())
}

// Output:
// 6 6 15
// 3 2 {2 1} [17 19]
// 16
// 212 3 18
// 37 98.6
// 9 10 10
