// phase: 08

// Escape analysis (step 15.2, ADR-114), the negative cases: every address here escapes (stored in a
// package variable, returned, appended, sent, converted to an interface, passed to a func value,
// captured by a closure, used by a goroutine or a defer, copied to another pointer variable...), so
// the object must stay on the heap and live as long as its last pointer. A variable kept on the
// stack by mistake would be a dangling pointer: wrong output or an ASan report. The output is what
// real Go prints.
package main

import (
	"fmt"
	"sync"
)

type Counter struct {
	n    int
	hist [4]int
}

func (c *Counter) Inc()           { c.n++ }
func (c *Counter) Self() *Counter { return c }
func (c *Counter) Keep()          { keep = c }
func (c Counter) Total() int      { return c.n }

type Incer interface{ Inc() }

var keep *Counter
var keepAll []*Counter
var keepInt *int
var ch = make(chan *Counter, 4)

func store(c *Counter)                     { keep = c }
func storeVia(c *Counter)                  { store(c) }
func storeInt(p *int)                      { keepInt = p }
func ident(c *Counter) *Counter            { return c }
func appendTo(c *Counter)                  { keepAll = append(keepAll, c) }
func send(c *Counter)                      { ch <- c }
func toIface(c *Counter) Incer             { return c }
func viaIface(i Incer)                     { i.Inc() }
func viaFunc(f func(*Counter), c *Counter) { f(c) }
func fieldOf(c *Counter) *int              { return &c.n }
func sliceOf(c *Counter) []int             { return c.hist[:] }
func nested(c *Counter) *Counter           { d := c; return d }
func gen[T any](p *T) *T                   { return p }

func retAddr() *Counter {
	var c Counter
	c.n = 7
	return &c
}

func retNew() *Counter {
	p := &Counter{n: 8}
	return p
}

func retNew2() *Counter {
	p := new(Counter)
	p.n = 9
	return p
}

var keepFn func() int

func main() {
	// stored in a global
	var a Counter
	a.n = 1
	store(&a)
	a.n = 2
	fmt.Println(keep.n)

	// transitively stored
	var b Counter
	b.n = 3
	storeVia(&b)
	b.Inc()
	fmt.Println(keep.n)

	// a method that keeps its receiver
	var c Counter
	c.Keep()
	c.Inc()
	fmt.Println(keep.n)

	// returned by a callee
	var d Counter
	d.n = 4
	e := ident(&d)
	e.Inc()
	fmt.Println(d.n, e.n)

	// returned address of a local
	f := retAddr()
	f.Inc()
	g := retNew()
	g.Inc()
	h := retNew2()
	h.Inc()
	fmt.Println(f.n, g.n, h.n)

	// appended to a slice
	for i := 0; i < 3; i++ {
		var l Counter
		l.n = i
		appendTo(&l)
	}
	var appended []int
	for _, x := range keepAll {
		appended = append(appended, x.n)
	}
	fmt.Println(appended)

	// sent on a channel
	var s Counter
	s.n = 5
	send(&s)
	s.Inc()
	fmt.Println((<-ch).n)

	// converted to an interface
	var i Counter
	var inc Incer = &i
	inc.Inc()
	viaIface(&i)
	toIface(&i).Inc()
	fmt.Println(i.n)

	// passed to a func value
	var j Counter
	viaFunc(func(c *Counter) { c.Inc(); keep = c }, &j)
	j.Inc()
	fmt.Println(j.n, keep.n)

	// captured by a closure
	var k Counter
	inck := func() { k.Inc() }
	inck()
	inck()
	fmt.Println(k.n)

	// address inside a closure
	var m Counter
	pm := &m
	incm := func() { pm.Inc() }
	incm()
	fmt.Println(m.n)

	// goroutine
	var wg sync.WaitGroup
	var n Counter
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.Inc()
	}()
	wg.Wait()
	fmt.Println(n.n)

	// defer
	func() {
		var o Counter
		defer o.Inc()
		defer func() { fmt.Println("deferred", o.n) }()
		o.n = 10
	}()

	// method value
	var q Counter
	mv := q.Inc
	mv()
	mv()
	fmt.Println(q.n)

	// address of a field kept
	var r Counter
	storeInt(&r.n)
	*keepInt = 11
	fmt.Println(r.n)
	pf := fieldOf(&r)
	*pf = 12
	fmt.Println(r.n)

	// slice of an array in the variable
	var t Counter
	sl := sliceOf(&t)
	sl[1] = 13
	fmt.Println(t.hist)
	sl2 := t.hist[:]
	sl2[2] = 14
	fmt.Println(t.hist)

	// pointer variable copied
	var u Counter
	pu := &u
	pu2 := pu
	pu2.Inc()
	pu.Inc()
	fmt.Println(u.n)

	// pointer variable reassigned
	var v1, v2 Counter
	pv := &v1
	pv.Inc()
	pv = &v2
	pv.Inc()
	pv.Inc()
	fmt.Println(v1.n, v2.n)

	// generic callee
	var w Counter
	pw := gen(&w)
	pw.Inc()
	fmt.Println(w.n)

	// a new object held and then stored
	p1 := &Counter{n: 20}
	store(p1)
	p1.Inc()
	fmt.Println(keep.n)

	// a new object, a method that returns its receiver
	p2 := (&Counter{n: 30}).Self()
	p2.Inc()
	fmt.Println(p2.n)

	// a struct holding a pointer to a local
	type holder struct{ p *Counter }
	var x1 Counter
	hd := holder{p: &x1}
	hd.p.Inc()
	fmt.Println(x1.n, hd.p.n)

	// the address in a map and an array
	var x2 Counter
	mm := map[string]*Counter{"a": &x2}
	mm["a"].Inc()
	arr := [2]*Counter{&x2, &x2}
	arr[1].Inc()
	fmt.Println(x2.n)

	// read by a closure that escapes, changed through a method: the closure shares the variable
	var y Counter
	keepFn = func() int { return y.n }
	y.Inc()
	y.Inc()
	fmt.Println(keepFn(), y.n)
}

// Output:
// 2
// 4
// 1
// 5 5
// 8 9 10
// [0 1 2]
// 6
// 3
// 2 2
// 2
// 1
// 1
// deferred 10
// 2
// 11
// 12
// [0 13 0 0]
// [0 13 14 0]
// 2
// 1 2
// 1
// 21
// 31
// 1 1
// 2
// 2 2
