// phase: 04
// ExitCode: 2
// Stderr: ^panic: C\+\+ exception std::out_of_range: [^\n]*\n\ngoroutine 1 \[running\]:\nvec_at\([^\n]*\)\n\t/.+/cxxvals\.hpp:\d+ \+0x[0-9a-f]+\nmain\.at\(\.\.\.\)\n\t/.+/cxx_exception\.gp:24 \+0x[0-9a-f]+\nmain\.inner\(\.\.\.\)\n\t/.+/cxx_exception\.gp:31 \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/cxx_exception\.gp:36 \+0x[0-9a-f]+\n$

// A C++ exception that nothing catches ends the program like an unrecovered Go panic (Phase 04
// README §2.8, first version): "panic: C++ exception <type>: <what()>", the stack where it was
// thrown (the C++ helper's frame too), exit code 2. In this first version the deferred calls
// don't run (Go would run them for a panic): the C++ library looks for a catch before it unwinds
// anything, finds none, and ends the program there; Phase 05 makes the exception a Go panic that
// runs them and that recover can stop.
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
