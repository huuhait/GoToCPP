// phase: 04
// ExitCode: 2
// Stderr: ^panic: stop in a C thread\n\ngoroutine \d+ \[running\]:\nmain\.visit\(\.\.\.\)\n\t/.+/callback_thread_panic\.gp:\d+ \+0x[0-9a-f]+\n(.*\n)*$

// A callback that runs on a thread of C's own (not a goroutine) panics: the program ends like an
// unrecovered panic of any goroutine (step 4.11), with the thread's goroutine id.
package main

import (
	"fmt"
	"unsafe"

	"./cbtable.h"
)

func visit(user unsafe.Pointer, i C.int) C.int {
	panic("stop in a C thread")
}

func main() {
	defer fmt.Println("main defer: not reached")
	fmt.Println(cbtable.cb_in_thread(visit, nil, 1))
}

// Output:
