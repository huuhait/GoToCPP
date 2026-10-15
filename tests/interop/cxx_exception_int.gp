// phase: 04
// ExitCode: 2
// Stderr: ^panic: C\+\+ exception of type int\n\ngoroutine 1 \[running\]:\nthrow_int\(int\)\n\t/.+/cxxvals\.hpp:\d+ \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/cxx_exception_int\.gp:18 \+0x[0-9a-f]+\n$

// A C++ exception that is not a std::exception (throw 42) has no what(): the report names its
// type.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func main() {
	cxxvals.throw_int(0)
	fmt.Println("no throw")
	cxxvals.throw_int(42)
}

// Output:
// no throw
