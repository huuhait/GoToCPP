// phase: 05
// ExitCode: 2
// Stderr: ^panic: boom\n\tpanic: C\+\+ exception std::runtime_error: bad thing 1\n\ngoroutine 1 \[running\]:\npanic\(\.\.\.\)\n\t\?:0\nmain\.main\(\)\n\t/.+/cxx_exception_defer\.gp:22 \+0x[0-9a-f]+\n$

// A deferred C++ call that throws while a Go panic runs: the exception becomes a panic of the
// deferred call (Phase 05 step 5.10), as for a panic in a deferred call, Go's report shows both,
// the first one first, and the trace shows the deferred call on top of the panicking function. The
// other deferred calls still run, also those deferred before it.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func main() {
	defer fmt.Println("still runs")
	defer cxxvals.throw_runtime(1)
	defer fmt.Println("deferred")
	fmt.Println("start")
	panic("boom")
}

// Output:
// start
// deferred
// still runs
