// phase: 08

// Borrowed pointers loaded from elements of slices, arrays, maps and fields that the callee overwrites (ADR-086).
package main

import "fmt"

type E struct {
	n    int
	name string
	pad  [4]int
}

var gs []E
var gsp []*E
var garr [4]E
var gm = map[int]*E{}

type Holder struct {
	items []E
	ptrs  []*E
	one   E
	arr   [3]E
}

var gh Holder
var gph *Holder

func (e *E) grow() int {
	// append reallocates the backing array: the old one is freed if the receiver was lent from it
	gs = append(gs, E{n: 100}, E{n: 101}, E{n: 102}, E{n: 103}, E{n: 104})
	gs = nil
	junk := make([]E, 64)
	for i := range junk {
		junk[i].n = -1
	}
	e.n++
	e.pad[1] = e.n
	return e.n + len(e.name) + e.pad[1]
}

func (e *E) shrink() int {
	gs = gs[:0]
	gs = nil
	junk := make([]E, 64)
	_ = junk
	return e.n + len(e.name)
}

func (e *E) dropHolder() int {
	gh.items = nil
	gph = nil
	junk := make([]E, 64)
	_ = junk
	e.n += 2
	return e.n + len(e.name)
}

func (e *E) dropPtrs() int {
	gh.ptrs = nil
	gsp = nil
	junk := make([]*E, 64)
	for i := range junk {
		junk[i] = &E{n: -1, name: "xxxxxxxx"}
	}
	e.n += 3
	return e.n + len(e.name)
}

func (e *E) moveElem() int {
	// swap the element out: the slice element is overwritten with a different one
	gs[0], gs[1] = gs[1], gs[0]
	e.n += 5
	return e.n + len(e.name)
}

func fill() {
	gs = []E{{n: 1, name: "e0"}, {n: 2, name: "e1"}}
	gsp = []*E{{n: 3, name: "p0"}, {n: 4, name: "p1"}}
	garr = [4]E{{n: 5, name: "a0"}}
	gm = map[int]*E{1: {n: 6, name: "m1"}}
	gh = Holder{items: []E{{n: 7, name: "h0"}}, ptrs: []*E{{n: 8, name: "hp0"}}, one: E{n: 9, name: "one"}}
	gph = &Holder{items: []E{{n: 10, name: "ph0"}}, ptrs: []*E{{n: 11, name: "php0"}}, one: E{n: 12, name: "pone"}}
	gph.arr[1] = E{n: 13, name: "parr1"}
}

func main() {
	fill()
	fmt.Println(gs[0].grow())
	fill()
	fmt.Println(gs[1].shrink())
	fill()
	fmt.Println(gh.items[0].dropHolder())
	fill()
	fmt.Println(gph.items[0].dropHolder())
	fill()
	fmt.Println(gsp[0].dropPtrs())
	fill()
	fmt.Println(gh.ptrs[0].dropPtrs())
	fill()
	fmt.Println(gph.ptrs[0].dropPtrs())
	fill()
	fmt.Println(gm[1].dropPtrs())
	fill()
	fmt.Println(gs[0].moveElem())
	fill()
	fmt.Println(gh.one.dropHolder())
	fill()
	fmt.Println(gph.one.dropHolder())
	fill()
	fmt.Println(gph.arr[1].dropHolder())
	fill()
	fmt.Println(garr[0].grow())
	fill()
	p := &gs[1]
	fmt.Println(p.grow())
	fill()
	q := gph
	fmt.Println(q.items[0].dropHolder(), q.one.n)
	fill()
	hp := &gph.items[0]
	fmt.Println(hp.dropHolder())
	fill()
	hs := gs
	fmt.Println(hs[0].grow(), len(hs))
}

// Output:
// 6
// 4
// 11
// 15
// 8
// 14
// 18
// 11
// 9
// 14
// 18
// 20
// 14
// 8
// 15 12
// 15
// 6 2
