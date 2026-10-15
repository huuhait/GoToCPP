// phase: 04

// A call whose result is used as an object (its member or element) and that must go into a
// temporary first, for Go's order of evaluation, is a reference (auto&&) to that object. Its own
// operands go into named temporaries before it, so that a reference that the C++ function returns
// into them (std::max of two temporaries, append on a temporary string returns *this, a function
// that returns its const& argument) doesn't dangle: an ASan run checks it. A place (an element, a
// member) is no temporary: it goes to the function as itself.
package main

import (
	"fmt"

	"./cvals.h"
	"<algorithm>"
	"<string>"
)

func trace(s string, n int) int {
	fmt.Println("eval", s)
	return n
}

func main() {
	fmt.Println(int(std.max(std.string("apple"), std.string("banana"))[0]) + trace("a", 1))
	fmt.Println(int(std.string("abc").append("d")[3]) + trace("b", 1))
	fmt.Println(int(cvals.same_bag(cvals.make_bag()).items[2]) + trace("c", 1))
	fmt.Println(int(std.max(std.string("apple"), std.string("banana")).at(trace("d", 1))))
	fmt.Println(int(std.string("abc").append("d").at(trace("e", 3))))
	fmt.Println(int(cvals.same_bag(cvals.make_bag()).items.at(trace("f", 1))))
	n := 5
	fmt.Println(int(std.max(n, int(0))) + trace("g", 1))
	// A place goes to a C++ function as itself: pick takes and returns references, and the
	// store goes into cvals.pts[1].
	cvals.pick(cvals.pts[0], cvals.pts[1]).x = C.int(trace("h", 50))
	a, b := C.int(3), C.int(7)
	fmt.Println(int(cvals.pts[0].x), int(cvals.pts[1].x), int(std.min(a, b))+trace("i", 0))
}

// Output:
// eval a
// 99
// eval b
// 101
// eval c
// 4
// eval d
// 97
// eval e
// 100
// eval f
// 2
// eval g
// 6
// eval h
// eval i
// 1 50 3
