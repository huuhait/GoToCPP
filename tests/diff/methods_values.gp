// phase: 03

// Method values and method expressions next to calls and changes (step 3.10b): the receiver is
// read where the value is made, after the calls of its scope (g.M next to f(), which changes g,
// binds the new g), a call in it first; a value receiver is copied then (later changes don't
// reach it, also through the pointer it came from), a pointer receiver is shared; method values
// of named slice and func types, of variadic methods and of a receiver larger than 128 KiB; method
// expressions with a value, a pointer and several results as the arguments, stored and passed;
// the values in structs, slices and maps, called later and deferred, and a method value that
// drops the last pointer to its receiver's object while it runs.
package main

import "fmt"

type T struct{ v int }

func (t T) M() int           { return t.v }
func (t *T) PM() int         { t.v++; return t.v }
func (t T) Add(a, b int) int { return t.v + a + b }
func (t T) Sum(xs ...int) int {
	s := t.v
	for _, x := range xs {
		s += x
	}
	return s
}
func (t T) Drop() int { g = nil; return t.v }

var g = &T{1}
var gv = T{10}
var gs = []T{{100}}

func f() int       { fmt.Println("f"); g = &T{2}; gv = T{20}; gs = []T{{200}}; return 0 }
func mk() *T       { fmt.Println("mk"); return &T{3} }
func id(n int) int { fmt.Println("id", n); return n }

func h(a func() int, b int, c func() int) { fmt.Println(a(), b, c()) }

func call(f func() int) int { return f() }

type IDs []int

func (ids IDs) Len() int    { return len(ids) }
func (ids *IDs) Push(v int) { *ids = append(*ids, v) }

type F func(int) int

func (f F) Twice(x int) int { return f(f(x)) }

type Big struct {
	a [20000]int
	n int
}

func (b Big) N() int { return b.n + len(b.a) }

type Box struct {
	get func() int
	put func() int
}

func three() (T, int, int) { return T{30}, 1, 2 }

func main() {
	h(g.M, f(), gv.M)
	g = &T{1}
	gv = T{10}
	h(g.PM, f(), gv.PM)
	h(gs[0].M, f(), mk().M)
	fmt.Println(id(1), call(mk().PM), id(2) > 0)

	x := T{5}
	p := &x
	m, pm, pv := x.M, x.PM, p.M
	x.v = 7
	fmt.Println(m(), pm(), pv(), x.v, m != nil)
	add, sum := x.Add, p.Sum
	fmt.Println(add(1, 2), sum(), sum(1, 2), T.Add(x, 3, 4), T.Add(three()), (*T).Sum(p, 5, 6))
	e, pe := T.M, (*T).PM
	fmt.Println(e(x), pe(p), (*T).M(p))

	ids := IDs{1}
	push, n := ids.Push, ids.Len
	push(2)
	push(3)
	fmt.Println(n(), ids.Len(), IDs.Len(ids))

	var tr F = func(i int) int { return i + 1 }
	tw := tr.Twice
	tr = nil
	fmt.Println(tw(1), F.Twice(func(i int) int { return i * 2 }, 3))

	var big Big
	big.n = 3
	bn := big.N
	big.n = 4
	fmt.Println(bn(), Big.N(big))

	b := Box{get: gv.M, put: gv.PM}
	gv.v = 50
	fmt.Println(b.get(), b.put(), gv.v)
	fs := map[string]func() int{"x": x.M, "g": g.M}
	x.v = 0
	fmt.Println(fs["x"](), fs["g"]())

	g = &T{9}
	drop := g.Drop
	fmt.Println(drop(), g == nil)

	defer fmt.Println("deferred last")
	d := x.M
	defer d()
	defer func() { fmt.Println("closure", m(), d()) }()
	x.v = 1000
	defer fmt.Println(x.M(), pm())
}
