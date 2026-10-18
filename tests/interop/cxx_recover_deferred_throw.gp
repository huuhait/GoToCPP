// phase: 05

// A deferred C++ call that throws while a Go panic runs replaces that panic, as a deferred call
// that panics does: recover returns the exception, the other deferred calls still run.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func main() {
	defer func() {
		fmt.Println("recovered:", recover())
	}()
	defer fmt.Println("runs after the throw")
	defer cxxvals.throw_runtime(9)
	defer fmt.Println("runs before the throw")
	panic("go panic")
}

// Output:
// runs before the throw
// runs after the throw
// recovered: C++ exception std::runtime_error: bad thing 9
