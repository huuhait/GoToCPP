// phase: 05
// ExitCode: 2
// Stderr: ^panic: C\+\+ exception std::runtime_error: bad thing 5 \[recovered, repanicked\]\n\ngoroutine 1 \[running\]:\n

// Panicking again with the recovered value of a C++ exception is the same panic: printed once.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r)
		panic(r)
	}()
	cxxvals.throw_runtime(5)
}

// Output:
// recovered: C++ exception std::runtime_error: bad thing 5
