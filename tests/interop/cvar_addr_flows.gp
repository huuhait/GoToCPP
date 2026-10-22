// phase: 08

// How the address of a C/C++ local reaches the place where it is kept (ADR-139): through a chain
// of local objects in any order, through two variables that refer to the same object, through the
// address of a pointer variable, through copy, append and the go statement (a literal or a named
// function), through the alias of a parameter, two steps of pointers, a map or a pointer parameter, a function
// literal with a defer of its own, a range-over-func body, an interface and an unsafe.Pointer.
// The local is boxed in each, and nothing else keeps it alive: main overwrites the stack with
// clobber() and reads the values after all of them.
package main

import (
	"fmt"
	"iter"
	"unsafe"

	"./cvar_addr_flows.hpp"
)

type Holder struct {
	p    *C.int
	next *Holder
}

var gchain2 *Holder
var gchain3 *Holder
var gchain3b *Holder
var galias *Holder
var galias2 *Holder
var gobj *Holder
var gslice []*C.int
var gfill = &Holder{}
var gcopy []*C.int = make([]*C.int, 1)
var gcopy2 []*C.int
var gcap []*C.int = make([]*C.int, 0, 4)
var gcap2 []*C.int = make([]*C.int, 0, 4)
var gsl []*C.int
var garr []*C.int
var gstart = make(chan bool)
var gresA = make(chan int, 1)
var gresB = make(chan int, 1)
var gfilled = make(Holders, 1)
var gmap map[string]*Holder = map[string]*Holder{}
var grange []*C.int
var gresC = make(chan int, 1)
var gresD = make(chan int, 1)
var gresE = make(chan int, 1)
var gparam = &Holder{}
var gnode = &Holder{}
var gmap2 = map[string]*Holder{}
var gstar = &Holder{}
var giface any
var gunsafe unsafe.Pointer
var gnotes int

type Holders []Holder

func (p *Holders) Fill() { *p = gfilled }

func clobber() int {
	var a [64]int
	for i := range a {
		a[i] = i + 1000
	}
	return a[5]
}

// A chain whose last link is stored first: the address travels p1 -> p2 -> gchain2.
func chain2() {
	var x C.int = 1
	p1, p2 := &Holder{}, &Holder{}
	gchain2 = p2
	p2.next = p1
	p1.p = &x
}

func chain3() {
	var x C.int = 2
	p1, p2, p3 := &Holder{}, &Holder{}, &Holder{}
	gchain3 = p3
	p3.next = p2
	p2.next = p1
	p1.p = &x
}

// The chain in the forward order, stored last.
func chain3forward() {
	var x C.int = 3
	p1, p2, p3 := &Holder{}, &Holder{}, &Holder{}
	p1.p = &x
	p2.next = p1
	p3.next = p2
	gchain3b = p3
}

// Two variables for one object: the store goes through q, the object leaves through p.
func aliasStoreThenLeave() {
	var x C.int = 4
	p := &Holder{}
	q := p
	q.p = &x
	galias = p
}

func aliasReturned() *Holder {
	var x C.int = 5
	p := &Holder{}
	q := p
	q.p = &x
	return p
}

// A local struct and a pointer to it: the store through the pointer, the address of the struct leaves.
func addressOfLocalObject() {
	var x C.int = 6
	var h Holder
	p := &h
	p.p = &x
	gobj = &h
}

func sliceAlias() {
	var x C.int = 7
	s := make([]*C.int, 1)
	t := s
	t[0] = &x
	gslice = s
}

func fill(pp **Holder) { *pp = gfill }

// The pointer's own address goes to a function that stores another pointer into it.
func fillAddr() {
	var x C.int = 8
	var p *Holder
	fill(&p)
	p.p = &x
}

func copyIntoPackageSlice() {
	var x C.int = 9
	copy(gcopy, []*C.int{&x})
}

func copyIntoLocalSlice() {
	var x C.int = 10
	d := make([]*C.int, 1)
	copy(d, []*C.int{&x})
	gcopy2 = d
}

// append writes into the package slice's backing array when it has the capacity.
func appendIntoPackageArray() {
	var x C.int = 12
	s := append(gcap, nil)
	s[0] = &x
}

// The result of the append is dropped, but the elements went into the package slice's array.
func appendDiscarded() {
	var x C.int = 19
	s := append(gcap2, &x)
	_ = s
}

func sliceOfSlice() {
	var x C.int = 20
	s := make([]*C.int, 1)
	t := s[:]
	t[0] = &x
	gsl = s
}

func sliceOfArray() {
	var x C.int = 21
	var arr [2]*C.int
	t := arr[:]
	t[0] = &x
	garr = arr[:]
}

// The goroutine reads the local after main has overwritten the stack.
func goArgument() {
	var x C.int = 13
	p := &Holder{}
	p.p = &x
	go func(h *Holder) {
		<-gstart
		gresA <- int(*h.p)
	}(p)
}

func goCapture() {
	var x C.int = 14
	p := &Holder{}
	p.p = &x
	go func() {
		<-gstart
		gresB <- int(*p.p)
	}()
}

func mapPut() {
	var x C.int = 15
	h := &Holder{}
	h.p = &x
	gmap["k"] = h
}

// A method with a pointer receiver takes the slice variable's address: it may point anywhere after the call.
func implicitAddress() {
	var x C.int = 18
	s := make(Holders, 1)
	s.Fill()
	s[0].p = &x
}

func rangeOver() {
	var x C.int = 16
	local := []*C.int{&x}
	for _, e := range local {
		grange = append(grange, e)
	}
}

// The go statement with a named function: its arguments are read after main has overwritten the stack.
func goReadAddress(p *C.int) {
	<-gstart
	gresC <- int(*p)
}

// (it returns a value: the address in the arguments is still read by the goroutine)
func goReadPointer(p *C.int) int {
	<-gstart
	gresD <- int(*p)
	return 0
}

func goReadHolder(h Holder) {
	<-gstart
	gresE <- int(*h.p)
}

func goNamedAddress() {
	var x C.int = 22
	go goReadAddress(&x)
}

func goNamedPointer() {
	var x C.int = 23
	p := &x
	go goReadPointer(p)
}

func goNamedHolder() {
	var x C.int = 24
	h := Holder{p: &x}
	go goReadHolder(h)
}

// q is another name for the caller's object.
func paramAlias(param *Holder) {
	var x C.int = 25
	q := param
	q.p = &x
}

// Two steps through pointers: p is local, p.next is not.
func depthTwo() {
	var x C.int = 26
	p := &Holder{}
	p.next = gnode
	p.next.p = &x
}

func mapParam(m map[string]*Holder) {
	var x C.int = 27
	m["a"] = &Holder{p: &x}
}

func starParam(h *Holder) {
	var x C.int = 28
	*h = Holder{p: &x}
}

func cleanup() { gnotes++ }

// A function literal with a defer of its own: the variable it stores into is declared outside it, so
// it outlives the call, whatever the literal's frame holds.
func closureDeferVar() int {
	var cur *C.int
	f := func() {
		defer cleanup()
		var x C.int = 29
		cur = &x
	}
	f()
	clobber()
	return int(*cur)
}

func closureDeferStruct() int {
	var h Holder
	f := func() {
		defer cleanup()
		var x C.int = 30
		h.p = &x
	}
	f()
	clobber()
	return int(*h.p)
}

func closureDeferAppend() int {
	var list []*C.int
	add := func(v C.int) {
		defer cleanup()
		var x C.int = v
		list = append(list, &x)
	}
	add(31)
	add(32)
	clobber()
	return int(*list[0] + *list[1])
}

func closureDeferBlock() int {
	var cur *C.int
	f := func() {
		defer cleanup()
		for i := 0; i < 2; i++ {
			var x C.int = C.int(33 + i)
			cur = &x
		}
	}
	f()
	clobber()
	return int(*cur)
}

func closureInClosureDefer() int {
	var cur *C.int
	f := func() {
		defer cleanup()
		g := func() {
			defer cleanup()
			var x C.int = 35
			cur = &x
		}
		g()
	}
	f()
	clobber()
	return int(*cur)
}

func seq(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// The body of a range-over-func statement ends when the iterator's call of it returns, before the
// function (and its defer) does.
func rangeFuncDefer() int {
	defer cleanup()
	var cur *C.int
	for i := range seq(2) {
		var x C.int = C.int(36 + i)
		cur = &x
	}
	clobber()
	return int(*cur)
}

// Two range-over-func loops in a function with a defer: the second one stores.
func twoLoops() int {
	defer cleanup()
	n := 0
	for i := range seq(2) {
		n += i
	}
	var cur *C.int
	for i := range seq(2) {
		var x C.int = C.int(10 + i)
		cur = &x
	}
	clobber()
	return int(*cur) + n
}

func ifaceLeak() {
	var x C.int = 38
	var i any = &x
	giface = i
}

func unsafeLeak() {
	var x C.int = 39
	up := unsafe.Pointer(&x)
	gunsafe = up
}

func main() {
	chain2()
	chain3()
	chain3forward()
	aliasStoreThenLeave()
	back := aliasReturned()
	addressOfLocalObject()
	sliceAlias()
	fillAddr()
	copyIntoPackageSlice()
	copyIntoLocalSlice()
	appendIntoPackageArray()
	appendDiscarded()
	sliceOfSlice()
	sliceOfArray()
	goArgument()
	goCapture()
	mapPut()
	rangeOver()
	implicitAddress()
	goNamedAddress()
	goNamedPointer()
	goNamedHolder()
	paramAlias(gparam)
	depthTwo()
	mapParam(gmap2)
	starParam(gstar)
	ifaceLeak()
	unsafeLeak()
	clobber()
	fmt.Println(*gchain2.next.p, *gchain3.next.next.p, *gchain3b.next.next.p)
	fmt.Println(*galias.p, *back.p, *gobj.p, *gslice[0])
	fmt.Println(*gfill.p, *gcopy[0], *gcopy2[0], *gcap[:1][0])
	for i := 0; i < 5; i++ {
		gstart <- true
	}
	fmt.Println(<-gresA, <-gresB, *gmap["k"].p, *grange[0], *gfilled[0].p)
	fmt.Println(*gcap2[:1][0], *gsl[0], *garr[0])
	fmt.Println(<-gresC, <-gresD, <-gresE)
	fmt.Println(*gparam.p, *gnode.p, *gmap2["a"].p, *gstar.p)
	fmt.Println(closureDeferVar(), closureDeferStruct(), closureDeferAppend(), closureDeferBlock(), closureInClosureDefer(), rangeFuncDefer(), twoLoops())
	fmt.Println(*giface.(*C.int), *(*C.int)(gunsafe))
}

// Output:
// 1 2 3
// 4 5 6 7
// 8 9 10 12
// 13 14 15 16 18
// 19 20 21
// 22 23 24
// 25 26 27 28
// 29 30 63 34 35 37 12
// 38 39
