// phase: 05
// ExitCode: 2
// Stderr: ^panic: kept\n\ngoroutine 1 \[running\]:\n

// A Go panic that C++ code kept (std::current_exception, xp.hpp) and that a recover() stopped is
// thrown again in a function whose only deferred call has no recover(): the deferred call runs and
// the panic goes on, and ends the program (it is not taken for recovered again).
package main

import (
	"fmt"

	"./xp.hpp"
)

func noDefer() {
	xp.call_save(func() C.int {
		panic("kept")
	})
}

func plainDefer() {
	defer fmt.Println("plainDefer's deferred call (no recover)")
	xp.rethrow_saved()
}

func main() {
	func() {
		defer func() { fmt.Println("first recover:", recover()) }()
		noDefer()
	}()
	plainDefer()
	fmt.Println("not reached: the panic ends the program")
}

// Output:
// first recover: kept
// plainDefer's deferred call (no recover)
