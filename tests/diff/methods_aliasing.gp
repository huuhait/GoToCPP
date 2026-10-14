// phase: 03

// A value receiver is the method's own copy (Phase 03 README §2.5, the audit): writes that the
// method makes through other pointers (to the package variable it was called on, to the variable
// itself through a pointer argument, to the slice element it came from) don't change it, and
// the receiver stays valid when the method drops the last pointer to the object it was called on
// (g.Drop sets g to nil). A pointer receiver sees every write.
package main

import "fmt"

type Acc struct {
	n    int
	tags []string
}

var global = Acc{n: 1}
var g = &Acc{n: 2, tags: []string{"a"}}
var gs = []Acc{{n: 3}, {n: 4}}

func (a Acc) Show() int {
	global.n = 100
	return a.n
}

func (a Acc) Via(p *Acc) int {
	p.n = 7
	p.tags = nil
	return a.n*10 + len(a.tags)
}

func (a Acc) Drop() int {
	g = nil
	gs = append([]Acc(nil), Acc{n: 50})
	return a.n + len(a.tags)
}

func (a *Acc) Bump() int {
	global.n++
	return a.n
}

func (a Acc) Elem() int {
	gs[0].n = 99
	return a.n
}

func (a Acc) Self(b Acc) int {
	a.n += b.n
	return a.n
}

func main() {
	fmt.Println(global.Show(), global.n)
	x := Acc{n: 2, tags: []string{"t"}}
	fmt.Println(x.Via(&x), x.n, len(x.tags))
	fmt.Println(g.Drop(), g == nil, gs[0].n)
	gs = []Acc{{n: 3}, {n: 4}}
	fmt.Println(gs[0].Elem(), gs[0].n)
	global.n = 5
	p := &global
	fmt.Println(p.Bump(), global.Bump(), global.n)
	fmt.Println(x.Self(x), x.n)
	y := &x
	fmt.Println(y.Self(*y), y.n)
}
