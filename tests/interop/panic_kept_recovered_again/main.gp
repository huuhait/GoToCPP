// phase: 05

// A Go panic that C++ code kept (std::current_exception, xp.hpp) and that a recover() stopped is
// thrown again by the C++ code: it is a new panic in flight. recover() stops it again and returns
// its value, also after it went through a function with a deferred call
// (defer_frame::gp_take forgets that it was recovered before).
package main

import (
	"fmt"

	"./xp.hpp"
)

var count int

func passThrough() {
	defer func() { count++ }()
	xp.call_save(func() C.int {
		panic("orig value")
	})
}

func noDefer() {
	xp.call_save(func() C.int {
		panic("second value")
	})
}

func main() {
	// A: kept without a function with a deferred call on the way; B: through one.
	func() {
		defer func() { fmt.Println("A recover:", recover()) }()
		noDefer()
	}()
	func() {
		defer func() { fmt.Println("A again:", recover()) }()
		xp.rethrow_saved()
	}()
	func() {
		defer func() { fmt.Println("B recover:", recover()) }()
		passThrough()
	}()
	func() {
		defer func() { fmt.Println("B again:", recover()) }()
		xp.rethrow_saved()
	}()
	fmt.Println("count", count)
}

// Output:
// A recover: second value
// A again: second value
// B recover: orig value
// B again: orig value
// count 1
