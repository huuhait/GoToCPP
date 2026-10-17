// phase: 04

// A function with a defer holds the box of a C++ local whose address it takes until the deferred calls
// ran, but only where the pointer can outlive the local's block (step 4.8): the local is in the
// function's own block, or its address is stored in a variable of an outer block. A pointer that is
// only passed to a call, or kept in the same block, dies with the block, as in C++: an RAII object
// whose address goes to a function is destroyed at the end of its iteration (a lock guard
// held to the end of the function could deadlock), and a loop that takes the address of a local in
// every iteration doesn't grow with the iterations.
package main

import (
	"fmt"

	"./objects.h"
	"<string>"
)

func use(g *objects.Guard) { g.note("in use") }

func touch(l *objects.Live) { l.inc() }

// The Guard is destroyed at the end of each iteration, before the deferred call.
func raii() {
	defer fmt.Println("deferred")
	for i := 0; i < 2; i++ {
		g := objects.Guard(std.string("g"))
		use(&g)
		fmt.Println("end of iteration", i)
	}
	fmt.Println("after loop")
}

// Nothing is held for a pointer that is passed to a call or kept in the block: at most one Live
// exists at a time, however many iterations there are.
func bounded() {
	defer fmt.Println("bounded deferred")
	peak := 0
	for i := 0; i < 20000; i++ {
		a := objects.Live{}
		p := &a
		touch(p)
		if n := int(objects.live_count()); n > peak {
			peak = n
		}
	}
	fmt.Println("peak", peak, "after", int(objects.live_count()))
}

// A pointer stored in a variable of the function's block outlives the iteration: each iteration's
// object is held until the function ends (memory grows with such iterations, as Go's heap does).
func escaping() {
	var ps []*objects.Live
	defer func() { fmt.Println("escaping deferred", len(ps), int(objects.live_count())) }()
	for i := 0; i < 5; i++ {
		a := objects.Live{}
		a.inc()
		ps = append(ps, &a)
	}
	fmt.Println("escaping body", int(objects.live_count()))
}

// A local of the function's own block is held until after the deferred calls, also when only its
// address is passed to a call.
func top() {
	g := objects.Guard(std.string("top"))
	use(&g)
	defer fmt.Println("top deferred")
}

func main() {
	raii()
	bounded()
	escaping()
	fmt.Println("live after escaping", int(objects.live_count()))
	top()
}

// Output:
// lock g
// g: in use
// end of iteration 0
// unlock g
// lock g
// g: in use
// end of iteration 1
// unlock g
// after loop
// deferred
// peak 1 after 0
// bounded deferred
// escaping body 5
// escaping deferred 5 5
// live after escaping 0
// lock top
// top: in use
// top deferred
// unlock top
