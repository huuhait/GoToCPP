// phase: 05

// Embedding named types that are not structs (the C++ type is a gp::newtype): a number with a
// value and a pointer method, a slice, a map and a func type, and an embedded pointer to one.
package main

import "fmt"

type Celsius float64

func (c Celsius) F() float64     { return float64(c)*9/5 + 32 }
func (c *Celsius) Set(v float64) { *c = Celsius(v) }

type IDs []int

func (s IDs) Len() int   { return len(s) }
func (s *IDs) Add(v int) { *s = append(*s, v) }

type Counts map[string]int

func (m Counts) Inc(k string) { m[k]++ }

type Op func(int) int

func (f Op) Twice(x int) int { return f(f(x)) }

type T struct {
	Celsius
	IDs
	Counts
	Op
	n int
}

type P struct {
	*Celsius
	*IDs
}

func main() {
	t := T{Counts: Counts{}, Op: func(x int) int { return x * 3 }}
	t.Set(100)
	t.Add(3)
	t.Add(4)
	t.Inc("a")
	t.Inc("a")
	t.Inc("b")
	fmt.Println(t.F(), t.Len(), t.Celsius, t.IDs, t.Counts, t.Twice(2), t.Op(5), t.n)
	var c Celsius = 10
	var ids IDs
	p := P{&c, &ids}
	p.Set(37)
	p.Add(8)
	p.Add(9)
	fmt.Println(c, ids, p.F(), p.Len())
	fmt.Println(*p.Celsius, *p.IDs)
}

// Output:
// 212 2 100 [3 4] map[a:2 b:1] 18 15 0
// 37 [8 9] 98.6 2
// 37 [8 9]
