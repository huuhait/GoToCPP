// Method values and method expressions (Phase 03 step 3.10b): x.M binds a copy of the receiver
// when it is made (*p for a value method through a pointer, which panics then for a nil p), x.PM
// binds the pointer (x is boxed), both read after the calls of their scope like a method call's
// receiver; T.M, (*T).PM and (*T).M take the receiver as their first parameter, and a call of one
// calls its lambda at once; method values of named types (also a named func type), of variadic
// methods and of a receiver larger than 128 KiB (a box); stored in variables, fields, slices and
// named func types, passed, compared with nil, called through a defer; and promoted methods.
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

var g = &T{1}
var gv = T{10}

func f() int { g = &T{2}; gv = T{20}; return 0 }

func h(a func() int, b int, c func() int) { fmt.Println(a(), b, c()) }

type F func(int) int

func (f F) Twice(x int) int { return f(f(x)) }

type Handler func() int

type Big struct {
	a [20000]int
	n int
}

func (b Big) N() int { return b.n + len(b.a) }

type Holder struct {
	fn func() int
}

func apply(f func(T, int, int) int, t T) int { return f(t, 1, 2) }

func three() (T, int, int) { return T{30}, 1, 2 }

type Inner struct{ v int }

func (i Inner) Get() int  { return i.v }
func (i *Inner) Inc() int { i.v++; return i.v }

type Outer struct {
	Inner
	p *Inner
}

func main() {
	h(g.M, f(), gv.M)
	g = &T{1}
	gv = T{10}
	h(g.PM, f(), gv.PM)
	fmt.Println(gv.v, g.v)

	x := T{5}
	m := x.M
	pm := x.PM
	x.v = 7
	fmt.Println(m(), pm(), x.v, m != nil, x.M == nil)
	add := x.Add
	fmt.Println(add(1, 2), T.Add(x, 3, 4), (*T).PM(&x), apply(T.Add, T{100}), T.Add(three()))
	e := (*T).M
	fmt.Println(e(&x))
	sum := x.Sum
	fmt.Println(sum(), sum(1, 2, 3), T.Sum(x, 4), (*T).Sum(&x, []int{5, 6}...))

	var ff F = func(i int) int { return i * 3 }
	tw := ff.Twice
	fmt.Println(tw(2), F.Twice(ff, 1))
	var hd Handler = x.M
	fmt.Println(hd())

	var big Big
	big.n = 3
	bn := big.N
	big.n = 4
	fmt.Println(bn(), Big.N(big))

	hold := Holder{fn: gv.M}
	gv.v = 99
	fmt.Println(hold.fn())
	fs := []func() int{x.M, g.M, gv.PM}
	for _, f := range fs {
		fmt.Println(f())
	}

	o := Outer{Inner{1}, &Inner{10}}
	get, inc, pinc := o.Get, o.Inc, o.p.Inc
	o.v = 50
	fmt.Println(get(), inc(), pinc(), Outer.Get(o), (*Outer).Inc(&o), o.v)

	defer fmt.Println("deferred last")
	d := x.M
	defer d()
	defer func() { fmt.Println("in closure", m()) }()
	x.v = 1000
	defer fmt.Println(x.M())
}
