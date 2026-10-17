// phase: 04

// A deferred call on a C++ object (step 4.8): the object outlives the C++ block that declared it
// (C++ destroys a local when its block ends, before the function's deferred calls run: calling
// clear() on a destroyed vector was an AddressSanitizer error in the audit), the arguments of the
// deferred call are evaluated when the defer statement runs, as in Go, and the calls run last
// first, also in a loop.
package main

import (
	"fmt"

	"./objects.h"
	"<vector>"
)

func lifetime() {
	v := std.vector[int]{1, 2, 3}
	defer v.clear()
	defer fmt.Println("size at defer:", int(v.size())) // evaluated now: 3
	v.push_back(4)
	fmt.Println("size at return:", int(v.size()))
}

func inLoop() {
	var c objects.Counter
	for i := 0; i < 3; i++ {
		defer fmt.Println("deferred", i, int(c.get()))
		defer c.add(C.int(i + 1))
		c.inc()
	}
	fmt.Println("loop done", int(c.get()))
	defer func() { fmt.Println("final", int(c.get())) }()
}

// Each iteration's object is its own.
func perIteration() {
	for i := 0; i < 3; i++ {
		g := objects.Guard(std.string("g"))
		_ = g
		var w objects.Widget = objects.Widget(C.int(i))
		defer func() { fmt.Println("widget", int(w.get())) }()
		w.set(C.int(i * 10))
	}
}

func addrArg() {
	var c objects.Counter
	defer func() { fmt.Println("after", int(c.n)) }()
	defer objects.bump(c)       // c is copied now: bump changes the copy
	defer objects.bump_ptr(&c) // &c is c itself: the frame keeps its box alive
	c.add(1)
}

func argsNow() {
	n := C.int(1)
	defer fmt.Println("n at defer:", int(n))
	var c objects.Counter
	defer fmt.Println("c at defer:", int(c.get()))
	c.add(5)
	n = 9
	_ = n
}

// The arguments of a deferred member call may use the object: they are evaluated now.
func selfArg() {
	v := std.vector[int]{1, 2}
	defer v.push_back(C.int(v.size()))
	defer fmt.Println("in defer:", int(v.size()))
	v.push_back(7)
	fmt.Println("in body:", int(v.size()))
	w := std.vector[int]{}
	defer func() { fmt.Println("w:", int(w.size())) }()
	defer w.push_back(C.int(w.size() + 5))
}

// A C++ callable (here a function pointer) is evaluated when the defer statement runs.
func callable() {
	f := objects.get_shout()
	defer f(1)
	f = nil
	defer fmt.Println("f is now nil:", f == nil)
	g := objects.get_shout()
	defer func() { g(3) }()
}

func main() {
	callable()
	selfArg()
	lifetime()
	inLoop()
	perIteration()
	addrArg()
	argsNow()
}

// Output:
// shout 3
// f is now nil: true
// shout 1
// in body: 3
// w: 1
// in defer: 2
// size at return: 4
// size at defer: 3
// loop done 3
// final 3
// deferred 2 2
// deferred 1 1
// deferred 0 0
// lock g
// unlock g
// lock g
// unlock g
// lock g
// unlock g
// widget 20
// widget 10
// widget 0
// after 101
// c at defer: 0
// n at defer: 1
