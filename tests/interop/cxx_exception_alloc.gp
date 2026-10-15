// phase: 04
// ExitCode: 2
// Stderr: ^fatal error: runtime: out of memory\n\ngoroutine 1 \[running\]:\narena_alloc\(long long\)\n\t/.+/cxxvals\.hpp:\d+ \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/cxx_exception_alloc\.gp:22 \+0x[0-9a-f]+\n$

// A std::bad_alloc that C++ code throws itself (here an arena allocator without room; also
// std::bad_array_new_length, new int[-1] with g++), which the runtime's new handler never sees,
// is Go's fatal error "runtime: out of memory", as gocpp's own allocations are, with the trace
// where it was thrown; nothing unwinds, so the deferred call doesn't run.
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
