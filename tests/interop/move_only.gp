// phase: 04

// Values that C++ can only move (std::unique_ptr) go into calls as they would in C++: a call
// argument is not copied into a temporary when nothing needs it there, and a temporary that Go's
// order of evaluation needs is moved into its one use.
package main

import (
	"fmt"

	"./cvals.h"
	"<memory>"
	"<utility>"
	"<vector>"
)

func trace(s string, n int) C.int {
	fmt.Println("eval", s)
	return C.int(n)
}

func main() {
	var ups std.vector[std.unique_ptr[C.int]]
	ups.push_back(std.make_unique[C.int](1))
	ups.emplace_back(std.make_unique[C.int](2))
	p := std.make_unique[C.int](3)
	ups.push_back(std.move(p))
	fmt.Println(int(ups.size()), int(*ups[1]), bool(p == nil))
	fmt.Println(int(cvals.take_ptr(std.make_unique[C.int](10), trace("x", 4))))
	q := std.make_unique[C.int](20)
	fmt.Println(int(cvals.take_ptr(std.move(q), trace("y", 5))), q == nil)
}

// Output:
// 3 2 true
// eval x
// 14
// eval y
// 25 true
