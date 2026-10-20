// phase: 08

// Pointer receivers and pointer parameters that the callee only uses are passed as raw pointers
// (ADR-086), so the caller must keep the object alive. Every case here has a callee that drops the
// last other reference to its receiver or argument while it runs, through a package variable, a
// field, a slice element, a map entry, a closure or an interface, and then goes on using it: the
// call must not free it (a crash or an ASan report is the failure).
package main

import "fmt"

type T struct {
	n    int
	name string
	next *T
}

var g *T
var holder struct{ p *T }
var items []*T
var table map[string]*T
var drop func()
var gi Counter

type Counter interface{ Bump() int }

// The method drops the package variable that held the receiver, then uses the receiver.
func (t *T) dropGlobal() int {
	g = nil
	t.n += 10
	return t.n + len(t.name)
}

func (t *T) dropField() int {
	holder.p = nil
	t.n += 20
	return t.n + len(t.name)
}

func (t *T) dropElem() int {
	items[0] = nil
	t.n += 30
	return t.n + len(t.name)
}

func (t *T) dropMap() int {
	delete(table, "k")
	t.n += 40
	return t.n + len(t.name)
}

func (t *T) dropClosure() int {
	drop()
	t.n += 50
	return t.n + len(t.name)
}

func (t *T) Bump() int {
	gi = nil
	t.n += 60
	return t.n + len(t.name)
}

// A pure method never drops anything, but its argument is loaded from a field: a chain.
func (t *T) sum() int {
	if t == nil {
		return 0
	}
	return t.n + t.next.sum()
}

// The same through a function with a pointer parameter, whose argument comes from a package
// variable, a field and an element; the function drops what it was called with.
func useAfterDrop(t *T, which int) int {
	switch which {
	case 0:
		g = nil
	case 1:
		holder.p = nil
	case 2:
		items[0] = nil
	case 3:
		table["k"] = nil
	}
	t.n++
	return t.n + len(t.name)
}

// A borrowed pointer is passed on: the caller of outer holds it, and so does inner's.
func outer(t *T) int { return inner(t) + inner(t) }

func inner(t *T) int {
	g = nil
	holder.p = nil
	t.n++
	return t.n
}

// A part of an object that a package variable, a field or an element holds: the receiver or the
// argument is the address of a field inside it, so the whole object must stay alive.
type Part struct{ v int }

type Whole struct {
	part Part
	name string
}

var gwhole *Whole
var wholes []*Whole

func (p *Part) dropWhole() int {
	gwhole = nil
	p.v++
	return p.v
}

func (p *Part) dropElem() int {
	wholes[0] = nil
	p.v += 2
	return p.v
}

func dropPart(p *Part) int {
	gwhole, wholes[0] = nil, nil
	p.v += 3
	return p.v
}

// The argument is the result of a call that returns the only reference.
func fresh(n int) *T { return &T{n: n, name: "fresh"} }

func (t *T) dropAll() int {
	g, holder.p, items[0] = nil, nil, nil
	t.n *= 2
	return t.n
}

func main() {
	g = &T{n: 1, name: "global"}
	fmt.Println(g.dropGlobal())

	holder.p = &T{n: 2, name: "field"}
	fmt.Println(holder.p.dropField())

	items = []*T{{n: 3, name: "elem"}}
	fmt.Println(items[0].dropElem())

	table = map[string]*T{"k": {n: 4, name: "map"}}
	fmt.Println(table["k"].dropMap())

	local := &T{n: 5, name: "closure"}
	drop = func() { local = nil }
	fmt.Println(local.dropClosure(), local == nil)

	gi = &T{n: 6, name: "iface"}
	fmt.Println(gi.Bump(), gi == nil)

	chain := &T{n: 1, next: &T{n: 2, next: &T{n: 3}}}
	fmt.Println(chain.sum())

	for which := 0; which < 4; which++ {
		g = &T{n: 7, name: "g"}
		holder.p = &T{n: 8, name: "h"}
		items = []*T{{n: 9, name: "i"}}
		table = map[string]*T{"k": {n: 10, name: "m"}}
		switch which {
		case 0:
			fmt.Println(useAfterDrop(g, which))
		case 1:
			fmt.Println(useAfterDrop(holder.p, which))
		case 2:
			fmt.Println(useAfterDrop(items[0], which))
		case 3:
			fmt.Println(useAfterDrop(table["k"], which))
		}
	}

	g = &T{n: 20, name: "outer"}
	holder.p = g
	fmt.Println(outer(g))
	g = &T{n: 30, name: "outer"}
	holder.p = g
	fmt.Println(outer(holder.p))

	// A method expression gets the variable as its argument: the method drops the variable.
	g = &T{n: 8, name: "expr"}
	fmt.Println((*T).dropGlobal(g), g == nil)
	g = &T{n: 9, name: "expr"}
	dg := (*T).dropGlobal
	fmt.Println(dg(g), g == nil)

	gwhole = &Whole{part: Part{1}, name: "w"}
	fmt.Println(gwhole.part.dropWhole(), gwhole == nil)
	wholes = []*Whole{{part: Part{2}}}
	fmt.Println(wholes[0].part.dropElem(), wholes[0] == nil)
	gwhole, wholes = &Whole{part: Part{3}}, []*Whole{{part: Part{4}}}
	fmt.Println(dropPart(&gwhole.part), gwhole == nil)
	gwhole, wholes = &Whole{part: Part{3}}, []*Whole{{part: Part{4}}}
	fmt.Println(dropPart(&wholes[0].part), wholes[0] == nil)

	g = &T{n: 1}
	holder.p = &T{n: 2}
	items = []*T{{n: 3}}
	fmt.Println(items[0].dropAll())
	fmt.Println(fresh(4).dropGlobal())
	fmt.Println(useAfterDrop(fresh(5), 9))
}

// Output:
// 17
// 27
// 37
// 47
// 62 true
// 71 true
// 6
// 9
// 10
// 11
// 12
// 43
// 63
// 22 true
// 23 true
// 2 true
// 4 true
// 6 true
// 7 true
// 6
// 19
// 11
