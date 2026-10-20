// phase: 08

// Borrowed pointers and closures: captured variables, closures that drop what they captured (ADR-086).
package main

import "fmt"

type T struct {
	n    int
	name string
}

var g *T
var h struct{ p *T }
var fns []func()
var drop func()
var seqs = map[string]func(func(*T) bool){}

func (t *T) viaClosure(f func()) int {
	f()
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	t.n++
	return t.n + len(t.name) + len(junk)
}

func viaClosureP(t *T, f func()) int {
	f()
	t.n++
	return t.n + len(t.name)
}

func twoParams(t *T, hh *struct{ p *T }) int {
	hh.p = nil
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return t.n + len(t.name)
}

type Dropper interface{ Drop() }
type D struct{}

func (D) Drop() { g = nil; h.p = nil }

func (t *T) viaIface(d Dropper) int {
	d.Drop()
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	return t.n + len(t.name)
}

func seq(yield func(*T) bool) {
	if !yield(g) {
		return
	}
	yield(h.p)
}

func (t *T) dropAll() int { g = nil; h.p = nil; t.n++; return t.n + len(t.name) }

func reset() {
	g = &T{n: 1, name: "one"}
	h.p = g
}

func main() {
	reset()
	fmt.Println(g.viaClosure(func() { g = nil; h.p = nil }))
	reset()
	fmt.Println(h.p.viaClosure(func() { g = nil; h.p = nil }))
	reset()
	drop = func() { g = nil; h.p = nil }
	fmt.Println(g.viaClosure(drop))
	reset()
	fmt.Println(viaClosureP(g, func() { g = nil; h.p = nil }))
	reset()
	fmt.Println(viaClosureP(h.p, drop))
	reset()
	fmt.Println(twoParams(h.p, &h))
	reset()
	fmt.Println(g.viaIface(D{}))
	reset()
	fmt.Println(h.p.viaIface(D{}))
	reset()
	for t := range seq {
		if t != nil {
			fmt.Println(t.dropAll())
		}
	}
	reset()
	for t := range seq {
		if t != nil {
			fmt.Println(g.dropAll())
			break
		}
		_ = t
	}
	reset()
	local := g
	fn := func() { local = nil; g = nil; h.p = nil }
	fmt.Println(local.viaClosure(fn))
	reset()
	lg := g
	fn2 := func() { lg = nil; g = nil; h.p = nil }
	fn2()
	fmt.Println(lg == nil)
	// the receiver is a closure-captured variable that the callee's closure reassigns
	reset()
	cv := g
	f3 := func() { cv = &T{n: 5, name: "other"} }
	fmt.Println(cv.viaClosure(f3), cv.n)
	reset()
	cv2 := g
	g, h.p = nil, nil
	fmt.Println(viaClosureP(cv2, func() { cv2 = nil }))
}

// Output:
// 15
// 15
// 15
// 5
// 5
// 4
// 4
// 4
// 5
// 5
// 15
// true
// 15 5
// 5
