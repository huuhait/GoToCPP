// phase: 03

// Methods of named types that aren't structs (step 3.10): a map, an array and a string type with
// value and pointer methods; a method named like a type, a blank and an unnamed receiver, a method
// that calls a package function named like a field, a pointer receiver that is boxed, and calls
// chained through a pointer method's result.
package main

import "fmt"

type Set map[string]bool

func (s Set) Add(k string)      { s[k] = true }
func (s Set) Has(k string) bool { return s[k] }
func (s *Set) Reset()           { *s = Set{} }

type Grid [3]int

func (g *Grid) Set(i, v int) { g[i] = v }
func (g Grid) Sum() int      { return g[0] + g[1] + g[2] }

type Name string

func (n Name) Twice() Name { return n + n }
func (n *Name) Append(s string) {
	*n += Name(s)
}

type Point struct{ X, Y int }

type T struct {
	v int
	f int
}

func f() int { return 41 }

func (t T) Point() Point { return Point{t.v, f()} }
func (_ T) Blank() int   { return 1 }
func (T) Unnamed() int   { return 2 }
func (t T) Call() int    { return f() + t.f }

func (p *Point) Boxed() int {
	pp := &p
	(*pp).X++
	return p.X
}

type Counter struct{ n int }

func (c *Counter) Inc() *Counter { c.n++; return c }
func (c Counter) Get() int        { return c.n }

func main() {
	s := Set{}
	s.Add("a")
	fmt.Println(s.Has("a"), s.Has("b"), len(s))
	s.Reset()
	fmt.Println(len(s))
	var g Grid
	g.Set(1, 5)
	fmt.Println(g.Sum(), g[1])
	n := Name("ab")
	n.Append("c")
	fmt.Println(n.Twice(), n)
	t := T{1, 2}
	fmt.Println(t.Point(), t.Blank(), t.Unnamed(), t.Call())
	p := &Point{1, 2}
	fmt.Println(p.Boxed(), *p)
	var c Counter
	c.Inc().Inc().Inc()
	fmt.Println(c.Get(), c.Inc().Get())
}
