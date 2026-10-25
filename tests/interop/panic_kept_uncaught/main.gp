// phase: 05
// ExitCode: 2
// Stderr: ^panic: orig value\n\ngoroutine 1 \[running\]:\n

// A Go panic that C++ code kept (std::current_exception, xp.hpp) goes on through a function with a
// deferred call, is recovered after the C++ code threw it again, and the C++ code throws it once
// more: nothing recovers it this time, and the report doesn't call it recovered ("panic: orig
// value", no "[recovered]"). (The second recover() of the Printf returns nil: the first one
// stopped the panic.)
package main

import (
	"fmt"

	"./xp.hpp"
)

func passThrough() {
	defer func() { fmt.Println("passThrough's deferred call") }()
	xp.call_save(func() C.int {
		panic("orig value")
	})
}

func main() {
	xp.swallow(func() C.int {
		passThrough()
		return 0
	})
	func() {
		defer func() { fmt.Printf("again: %v (%T)\n", recover(), recover()) }()
		xp.rethrow_saved()
	}()
	xp.rethrow_saved()
}

// Output:
// passThrough's deferred call
// swallowed
// again: orig value (<nil>)
