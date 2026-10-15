// phase: 04
// ExitCode: 2
// Stderr: ^panic: boom\n\tpanic: C\+\+ exception std::runtime_error: bad thing 1\n\ngoroutine 1 \[running\]:\nthrow_runtime\(int\)\n\t/.+/cxxvals\.hpp:\d+ \+0x[0-9a-f]+\npanic\(\.\.\.\)\n\t\?:0\nmain\.main\(\)\n\t/.+/cxx_exception_defer\.gp:21 \+0x[0-9a-f]+\n$

// A deferred C++ call that throws while a Go panic runs: as for a panic in a deferred call, Go's
// report shows both, the first one first, and the trace shows the deferred call on top of the
// panicking function. The deferred calls deferred before it don't run (first version).
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func main() {
	defer fmt.Println("not printed")
	defer cxxvals.throw_runtime(1)
	defer fmt.Println("deferred")
	fmt.Println("start")
	panic("boom")
}

// Output:
// start
// deferred
