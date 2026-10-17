// phase: 04

// The policy table of docs/spec/interop.md ("Types that can't be copied"): where Go's meaning
// can't be kept for a C++ type, the program doesn't compile, with an error at the .gp line. A
// copy of an object that C++ can't copy is C++'s own "use of deleted function"; the copies that
// gocpp makes itself (the arguments of a deferred call, a loop variable that a closure keeps,
// a result variable) are errors of the runtime's, which say what to do instead.
package main

import (
	"./objects.h"
	"<memory>"
	"<vector>"
)

// A copy of a move-only object: x := y, x = y, an argument.
func copies() {
	h := objects.make_handle(1)
	var g objects.Handle
	c := h              // ERROR "C\+\+: .*(deleted|copy constructor)"
	g = h               // ERROR "C\+\+: .*(deleted|copy assignment)"
	objects.take(h)     // ERROR "C\+\+: .*(deleted|copy constructor)"
	_, _ = c, g
}

// var x T needs a default constructor: Go's zero value must exist.
func zero() {
	var w objects.Widget // ERROR "C\+\+: .*(no matching|constructor)"
	_ = w
}

// range copies each element into the variable.
func rangeCopies() {
	var ups std.vector[std.unique_ptr[C.int]]
	for _, p := range ups { // ERROR "C\+\+: .*(deleted|copy)"
		_ = p
	}
}

// defer f(x) copies x when the defer statement runs.
func deferArg() {
	h := objects.make_handle(2)
	defer objects.take(h) // ERROR "C\+\+: defer copies the arguments of the deferred call"
}

// A 3-clause loop's variable is new for each iteration: a closure that keeps it needs a copy.
func loopVar() {
	for h := objects.make_handle(3); h.get() < 5; h = objects.make_handle(5) { // ERROR "C\+\+: a closure that uses the variable of a 3-clause for loop needs a copy"
		f := func() { _ = h.get() }
		f()
	}
}

func main() {
	copies()
	zero()
	rangeCopies()
	deferArg()
	loopVar()
}
