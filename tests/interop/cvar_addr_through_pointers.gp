// phase: 08

// The address of a C/C++ local that is stored through a pointer, slice or map that the function
// did not make keeps the local alive (ADR-139): the pointer may point into package data or into
// memory that the caller keeps, so Go's pointer would keep the variable alive. Each shape stores
// the address and nothing else keeps the local; main overwrites the stack with clobber() and
// reads the values after all of them. A pointer that the function made itself (a literal, new,
// &local) or an address that only goes to a call is not boxed (TestCAddrOwnership in
// internal/lower).
package main

import (
	"fmt"

	"./cvar_addr_through_pointers.hpp"
)

type Holder struct{ p *C.int }
type Box struct{ inner *Holder }

type Wrap struct{ in cav.Inner }

var gholder = &Holder{}
var gholders = []*Holder{{}, {}}
var gmap = map[string]*Holder{"k": {}}
var gany any = &Holder{}
var gptrs []*C.int = make([]*C.int, 2)
var gbox = Box{inner: &Holder{}}
var gch = make(chan *Holder, 1)
var gval Holder
var gparam = &Holder{}
var gresult []*C.int
var gcalled = &Holder{}
var gcomposite Holder
var gwrapfield *C.int
var gwrapelem *C.int
var gsend chan *C.int = make(chan *C.int, 1)
var galias = &Holder{}

func clobber() int {
	var a [64]int
	for i := range a {
		a[i] = i + 1000
	}
	return a[5]
}

func rangeAlias() {
	var x C.int = 1
	for _, h := range gholders {
		h.p = &x
	}
}

func commaOk() {
	var x C.int = 2
	h, ok := gmap["k"]
	if ok {
		h.p = &x
	}
}

func typeAssert() {
	var x C.int = 3
	h := gany.(*Holder)
	h.p = &x
}

func sliceExpr() {
	var x C.int = 4
	s := gptrs[:]
	s[0] = &x
}

func structHolds() {
	var x C.int = 5
	s := struct{ h *Holder }{gholder}
	s.h.p = &x
}

func closureParam() {
	var x C.int = 6
	func(h *Holder) { h.p = &x }(gbox.inner)
}

func chanRecv() {
	var x C.int = 7
	gch <- gval2()
	h := <-gch
	h.p = &x
}

var gval2h = &Holder{}

func gval2() *Holder { return gval2h }

func fieldChain() {
	var x C.int = 8
	h := gbox.inner
	h.p = &x
	gparam = h
}

func through(h *Holder) {
	var x C.int = 9
	h.p = &x
}

func get() *Holder { return gcalled }

func viaCall() {
	var x C.int = 10
	p := get()
	p.p = &x
}

func appended() []*C.int {
	var res []*C.int
	var x C.int = 11
	res = append(res, &x)
	return res
}

func composite() Holder {
	var x C.int = 12
	return Holder{p: &x}
}

func named() (h Holder) {
	var x C.int = 13
	h.p = &x
	return
}

func (w *Wrap) member() {
	gwrapfield = &w.in.b
}

func (w *Wrap) element() {
	var local = w
	gwrapelem = &local.in.a
}

func sent() {
	var x C.int = 15
	gsend <- &x
}

func returnedVar() *C.int {
	var x C.int = 16
	p := &x
	return p
}

func viaAlias() {
	var x C.int = 17
	h := galias
	k := h
	k.p = &x
}

func deref() {
	var x C.int = 14
	h := *gholder
	h.p = &x
	gval = h
}

func main() {
	rangeAlias()
	commaOk()
	typeAssert()
	sliceExpr()
	structHolds()
	closureParam()
	chanRecv()
	fieldChain()
	through(gparam)
	viaCall()
	gresult = appended()
	gcomposite = composite()
	n := named()
	(&Wrap{in: cav.Inner{a: 21, b: 22}}).member()
	(&Wrap{in: cav.Inner{a: 31, b: 32}}).element()
	viaAlias()
	sent()
	back := returnedVar()
	deref()
	clobber()
	fmt.Println(*gholders[0].p, *gholders[1].p, *gmap["k"].p, *gany.(*Holder).p)
	fmt.Println(*gptrs[0], *gholder.p, *gbox.inner.p, *gval2h.p)
	fmt.Println(*gparam.p, *gcalled.p, *gresult[0], *gcomposite.p, *n.p)
	fmt.Println(*gwrapfield, *gwrapelem, *gval.p, *<-gsend, *back, *galias.p)
}

// Output:
// 1 1 2 3
// 4 5 9 7
// 9 10 11 12 13
// 22 31 14 15 16 17
