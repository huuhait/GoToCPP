// phase: 03
// diff: stderr

// A method may drop the last pointer to the object it was called on (step 3.10): a value method
// has its copy (and the caller holds a package pointer for the call), a pointer method its owning
// pointer, also for an element of a slice that the method replaces and for a deferred call. Run
// with -asan too (tests/README.md: GOCPP_FLAGS).
package main

import "fmt"

type T struct {
	v  int
	s  []int
	nx *T
}

var g = &T{v: 1, s: []int{1, 2, 3}}
var gv = T{v: 2, s: []int{4}}
var gs = []T{{v: 3, s: []int{5}}}
var gps = []*T{{v: 4}}

func (t T) Drop() int {
	g = nil
	gv = T{}
	gs = nil
	gps = nil
	big := make([]int, 1000)
	_ = big
	return t.v + len(t.s)
}

func (t *T) DropP() int {
	g = &T{v: 100}
	gs = append([]T(nil), T{v: 9})
	gps[0] = nil
	return t.v + len(t.s)
}

func (t T) Self() int {
	t.nx = nil
	return t.v
}

func reset() {
	g = &T{v: 1, s: []int{1, 2, 3}}
	gv = T{v: 2, s: []int{4}}
	gs = []T{{v: 3, s: []int{5}}}
	gps = []*T{{v: 4}}
}

func drop() int { g = nil; gs = nil; return 0 }

func (t T) Arg(a int) int { return t.v + a }

func main() {
	fmt.Println(g.Drop())
	reset()
	fmt.Println(gv.Drop())
	reset()
	fmt.Println(gs[0].Drop())
	reset()
	fmt.Println(gps[0].Drop())
	reset()
	fmt.Println(g.DropP())
	reset()
	fmt.Println(gs[0].DropP())
	reset()
	fmt.Println(gps[0].DropP())
	reset()
	fmt.Println(gs[0].Arg(len(gs)))
	l := &T{v: 5, nx: &T{v: 6}}
	fmt.Println(l.nx.Self())
	n := l.nx
	l.nx = nil
	fmt.Println(n.Self())
	reset()
	defer fmt.Println("done")
	defer gs[0].Drop()
	defer g.Drop()
	drop()
}
