// phase: 04

// The policy table of docs/spec/interop.md ("Types that can't be copied"), row by row, with the
// types it names: a movable and copyable class (std::vector), a move-only one (Handle,
// std::unique_ptr), one with no default constructor (Widget) and one that can be neither copied
// nor moved (Pinned): what each construct of gocpp does with them where it works. The errors
// are policy_errors.gp's.
package main

import (
	"fmt"

	"./objects.h"
	"<memory>"
	"<utility>"
	"<vector>"
)

// make: a function result goes into a new variable in place (no copy, no move).
func newPinned(n int) objects.Pinned { return objects.Pinned(C.int(n)) }

func newHandle(n int) objects.Handle { return objects.make_handle(C.int(n)) }

func newWidget(n int) (objects.Widget, objects.Handle) {
	return objects.Widget(C.int(n)), objects.make_handle(C.int(n * 10))
}

// return a, b: each local that is returned once moves into the result.
func twoHandles() (objects.Handle, std.unique_ptr[C.int]) {
	a := objects.make_handle(30)
	b := std.make_unique[C.int](31)
	return a, b
}

func main() {
	// x := f() and var x T = f(): constructed in place, whatever the type can do
	a := newPinned(1)
	var b objects.Pinned = newPinned(2)
	w := newHandle(3)
	var u std.unique_ptr[C.int] = std.make_unique[C.int](4)
	fmt.Println(int(a.get()), int(b.get()), int(w.get()), int(*u))

	// var x T: the zero value is the default-constructed object; Widget has none, so it is
	// always built from arguments
	var h objects.Handle
	var v std.vector[int]
	var n std.unique_ptr[C.int]
	fmt.Println(int(h.get()), int(v.size()), n == nil)

	// x = f() assigns by move: a move-only type can be assigned from a temporary, and from
	// std.move(y)
	h = newHandle(5)
	w = std.move(h)
	fmt.Println(int(h.get()), int(w.get()))
	n = std.move(u)
	fmt.Println(n != nil, u == nil)

	// x := y copies a copyable object; the copy is a separate object
	v2 := std.vector[int]{1, 2}
	v3 := v2
	v3.push_back(3)
	fmt.Println(int(v2.size()), int(v3.size()))
	wid := objects.Widget(7)
	wid2 := wid
	wid2.set(8)
	fmt.Println(int(wid.get()), int(wid2.get()))

	// several results, each moved out of the tuple
	x, hh := newWidget(2)
	fmt.Println(int(x.get()), int(hh.get()))

	h1, u1 := twoHandles()
	fmt.Println(int(h1.get()), int(*u1))

	// a closure and a defer keep the object itself in its box: nothing is copied or moved
	p := newPinned(9)
	bump := func() { p.bump() }
	bump()
	defer p.bump()
	defer fmt.Println("pinned in defer", int(p.get()))

	// defer f(x) copies x now; a copyable one is fine, std.move(y) moves a move-only one
	hd := newHandle(11)
	defer objects.take(std.move(hd))
	fmt.Println("hd is empty", int(hd.get()))

	// parameters: a temporary or std.move(x) moves into a by-value parameter; defer f(g()) stores
	// the temporary
	objects.take(newHandle(12))
	h3 := newHandle(13)
	objects.take(std.move(h3))
	defer objects.take(newHandle(14))

	// range: the element is a copy; a move-only element is reached through the index
	ws := std.vector[objects.Widget]{}
	ws.push_back(objects.Widget(1))
	ws.push_back(objects.Widget(2))
	sum := 0
	for _, e := range ws {
		sum += int(e.get())
	}
	hs := std.vector[std.unique_ptr[C.int]]{}
	hs.push_back(std.make_unique[C.int](5))
	for i := range hs {
		sum += int(*hs[i])
	}
	fmt.Println(sum)
}

// Output:
// pinned 1
// pinned 2
// 1 2 3 4
// -1 0 true
// -1 5
// true true
// 2 3
// 7 8
// 2 20
// 30 31
// pinned 9
// hd is empty -1
// take 12
// close 12
// take 13
// close 13
// 8
// close 30
// close 20
// close 5
// unpinned 2
// unpinned 1
// take 14
// close 14
// take 11
// close 11
// pinned in defer 10
// unpinned 11
