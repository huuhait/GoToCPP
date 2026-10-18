// phase: 04
// ExitCode: 2
// Stderr: (?s)^panic: from a std::function\n\ngoroutine 1 \[running\]:\nmain\.main\.func1\(\)\n\t/.+/callback_cxx_panic\.gp:19 \+0x[0-9a-f]+\n.*cbx::in_scope.*main\.main\(\)\n\t/.+/callback_cxx_panic\.gp:18 \+0x[0-9a-f]+\n$

// Through C++ code a panic unwinds as it does through gocpp code (step 4.11): the C++
// destructors run ("leave"), and so do the deferred calls of the functions on the way. Only a C
// function pointer is a barrier (callback_panic.gp).
package main

import (
	"fmt"

	"./cbcxx.hpp"
)

func main() {
	defer fmt.Println("main defer")
	fmt.Println(cbx.in_scope(func() C.int {
		panic("from a std::function")
	}))
}

// Output:
// enter
// leave
// main defer
