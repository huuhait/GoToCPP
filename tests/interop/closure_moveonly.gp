// phase: 04

// Objects that C++ can't copy (std::unique_ptr, a type with deleted copy and move constructors,
// an RAII guard) in closures and defers (step 4.8): the box holds the object, built in place from
// its initializer, so it is never copied or moved; the closure copies only the box. A guard used by
// a deferred call is destroyed when the last deferred call is done, not when its block ends.
package main

import (
	"fmt"

	"./objects.h"
	"<memory>"
	"<utility>"
)

func uniquePtr() func() int {
	p := std.make_unique[C.int](41)
	return func() int {
		*p += 1
		return int(*p)
	}
}

func pinned() {
	m := objects.make_pinned(1) // built in place: no copy, no move
	bump := func() { m.bump() }
	bump()
	bump()
	fmt.Println(int(m.get()))
	var n objects.Pinned = objects.Pinned(5)
	defer n.bump() // the box is kept: n is not destroyed with its block
	defer fmt.Println("n is", int(n.get()))
}

func guard() {
	g := objects.Guard(std.string("a"))
	defer g.note("first deferred") // runs while g is alive
	defer fmt.Println("deferred print")
	g.note("body")
	h := objects.Guard(std.string("b"))
	release := func() { h.rename(std.string("b2")) }
	release()
	fmt.Println("end of guard()")
}

func handle() {
	h := objects.make_handle(7)
	get := func() int { return int(h.get()) }
	defer fmt.Println("get in defer", get())
	moved := std.move(h)
	fmt.Println(int(moved.get()), get())
}

func main() {
	next := uniquePtr()
	fmt.Println(next(), next())
	pinned()
	guard()
	handle()
	fmt.Println("done")
}

// Output:
// 42 43
// pinned 1
// 3
// pinned 5
// unpinned 3
// n is 5
// unpinned 6
// lock a
// a: body
// lock b
// end of guard()
// unlock b2
// deferred print
// a: first deferred
// unlock a
// 7 -1
// close 7
// get in defer 7
// done
