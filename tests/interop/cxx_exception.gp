// phase: 05
// ExitCode: 2
// Stderr: ^panic: C\+\+ exception std::out_of_range: [^\n]*\n\ngoroutine 1 \[running\]:\nmain\.at\(\.\.\.\)\n\t/.+/cxx_exception\.gp:\d+ \+0x[0-9a-f]+\nmain\.inner\(\.\.\.\)\n\t/.+/cxx_exception\.gp:\d+ \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/cxx_exception\.gp:\d+ \+0x[0-9a-f]+\n$

// A C++ exception that reaches a function with defers is a Go panic (Phase 05 step 5.10,
// docs/spec/interop.md "C++ exceptions"): the deferred calls of every frame run, and unrecovered
// it ends the program like any panic: "panic: C++ exception <type>: <what()>", the stack where the
// function with the defer caught it (the C++ helper's frame is gone: the exception has unwound it),
// exit code 2.
package main

import (
	"fmt"

	"<vector>"
	"./cxxvals.hpp"
)

func cleanup(name string) { fmt.Println("cleanup", name) }

func at(v std.vector[int], i int) int {
	defer cleanup("at")
	return int(cxxvals.vec_at(v, C.size_t(i)))
}

func inner(n int) {
	defer cleanup("inner")
	v := std.vector[int](3)
	fmt.Println("at 1:", at(v, 1))
	fmt.Println("at", n, at(v, n)) // throws std::out_of_range
}

func main() {
	defer cleanup("main")
	inner(5)
	fmt.Println("not reached")
}

// Output:
// cleanup at
// at 1: 0
// cleanup at
// cleanup inner
// cleanup main
