// phase: 04
// ExitCode: 2
// Stderr: ^panic: C\+\+ exception std::runtime_error: no more rows 2\n\ngoroutine 1 \[running\]:\n(.*\n)*main\.main\(\)\n\t/.+/callback_cxx_exception\.gp:\d+ \+0x[0-9a-f]+\n$

// A C++ exception that a callback lets out is as fatal as a panic is (step 4.11; step 4.12's
// message): it must not unwind through the C frames of cb_sum.
package main

import (
	"fmt"
	"unsafe"

	"./cbcxx.hpp"
	"./cbtable.h"
)

func visit(user unsafe.Pointer, i C.int) C.int {
	if i == 2 {
		return cbx.fail(i)
	}
	return i
}

func main() {
	defer fmt.Println("main defer: not reached")
	fmt.Println(cbtable.cb_sum(4, visit, nil))
}

// Output:
