// phase: 08

// Element references (step 15.2, ADR-114), the negative cases with interface calls: the region of
// the variable overwrites the slot (through a method, an interface call, the slice variable, a
// field, a global, an array that the slice aliases) and then uses the variable, so it must be a
// copy. A reference there would use a freed object (ASan) or the new value.
package main

import "fmt"

type Shape interface {
	Area() int
}

type Rect struct{ w, h int }

func (r *Rect) Area() int { return r.w * r.h }

type Reg struct{ items []Shape }

func (r *Reg) wipe() { r.items[0] = nil }

func (r *Reg) drop() { r.items = nil }

// Area on Evil clears the slot of the slice it sits in.
type Evil struct {
	reg *Reg
	n   int
}

func (e *Evil) Area() int {
	e.reg.items[0] = nil
	e.reg.items[1] = nil
	return e.n
}

var gs []Shape

func resetGlobal() { gs = nil }

func viaMethod(r *Reg) int {
	sum := 0
	for _, s := range r.items {
		r.wipe()
		if s != nil {
			sum += s.Area()
		}
	}
	return sum
}

func viaIface(r *Reg) int {
	v := r.items[0]
	return v.Area()
}

func viaRoot(xs []Shape) int {
	v := xs[0]
	xs = nil
	return v.Area()
}

func viaField(r *Reg) int {
	v := r.items[0]
	r.items = nil
	return v.Area()
}

func viaDrop(r *Reg) int {
	v := r.items[0]
	r.drop()
	return v.Area()
}

func viaArray() int {
	var arr [2]Shape
	arr[0] = &Rect{3, 3}
	xs := arr[:]
	v := xs[0]
	arr[0] = nil
	return v.Area()
}

func viaGlobal() int {
	v := gs[0]
	resetGlobal()
	return v.Area()
}

func viaPtr(r *Reg) int {
	p := r
	v := p.items[0]
	p = nil
	_ = p
	r.items = nil
	return v.Area()
}

func main() {
	r := &Reg{items: []Shape{&Rect{2, 3}, &Rect{1, 1}}}
	fmt.Println(viaMethod(r))
	r = &Reg{items: []Shape{&Rect{2, 3}, &Rect{1, 1}}}
	fmt.Println(viaRoot(r.items))
	e := &Evil{n: 7}
	e.reg = &Reg{items: []Shape{e, &Rect{1, 1}}}
	fmt.Println(viaIface(e.reg))
	r = &Reg{items: []Shape{&Rect{2, 3}, &Rect{1, 1}}}
	fmt.Println(viaField(r))
	r = &Reg{items: []Shape{&Rect{2, 3}, &Rect{1, 1}}}
	fmt.Println(viaDrop(r))
	fmt.Println(viaArray())
	gs = []Shape{&Rect{4, 4}}
	fmt.Println(viaGlobal())
	r = &Reg{items: []Shape{&Rect{5, 5}}}
	fmt.Println(viaPtr(r))
}

// Output:
// 7
// 6
// 7
// 6
// 6
// 9
// 16
// 25
