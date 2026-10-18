// phase: 05
// ExitCode: 2
// Stderr: ^fatal error: runtime: out of memory\n\ngoroutine 1 \[running\]:\nmain\.main\(\)\n\t/.+/cxx_exception_alloc\.gp:\d+ \+0x[0-9a-f]+\n$

// A std::bad_alloc that C++ code throws itself (here an arena allocator without room; also
// std::bad_array_new_length, new int[-1] with g++), which the runtime's new handler never sees,
// is Go's fatal error "runtime: out of memory", as gocpp's own allocations are, with the trace
// where the function with the defer caught it (without one, where it was thrown); a fatal error
// is not a panic: the deferred call doesn't run, and recover can't stop it.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func main() {
	defer fmt.Println("not printed")
	fmt.Println("start")
	_ = cxxvals.arena_alloc(16)
	fmt.Println("16 bytes")
	_ = cxxvals.arena_alloc(1 << 20)
}

// Output:
// start
// 16 bytes
