// phase: 05

// A Go panic that C++ code keeps (std::current_exception) on its way through a function with a
// deferred call: the deferred call runs and the panic goes on, and the exception that the C++ code
// kept still has the panic's value when it is thrown again, and recover() returns it. (The
// function lets the panic go on as a copy, defer_frame::gp_run: the caught exception may be shared
// by such std::exception_ptrs.)
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
		defer func() { fmt.Println("again:", recover()) }()
		xp.rethrow_saved()
	}()
	fmt.Println("done")
}

// Output:
// passThrough's deferred call
// swallowed
// again: orig value
// done
