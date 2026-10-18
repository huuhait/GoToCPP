// phase: 04
// ExitCode: 2
// Stderr: ^panic: stop at 2\n\ngoroutine 1 \[running\]:\nmain\.visit\(\.\.\.\)\n\t/.+/main\.gp:\d+ \+0x[0-9a-f]+\nctab_walk\([^\n]*\)\n\t/.+/ctable\.c:\d+ \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/main\.gp:\d+ \+0x[0-9a-f]+\n$

// A panic that leaves a callback never unwinds through the frames of a C library compiled
// without -fexceptions (step 4.11). The walk in ctable.c has a malloc'ed buffer, which a panic that
// unwound through it would skip freeing: the program ends at the callback instead, like an
// unrecovered panic, and runs no deferred call outside the C call.
package main

import (
	"fmt"
	"unsafe"

	"./ctable.h"
)

func visit(user unsafe.Pointer, i C.int) C.int {
	if i == 2 {
		panic("stop at 2")
	}
	fmt.Println("visit", int(i))
	return i
}

func main() {
	defer fmt.Println("main defer: not reached")
	fmt.Println(ctable.ctab_walk(5, visit, nil))
}

// Output:
// visit 0
// visit 1
