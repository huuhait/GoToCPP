// phase: 04
// ExitCode: 2
// Stderr: ^panic: stop at 2\n\ngoroutine 1 \[running\]:\nmain\.visit\(\.\.\.\)\n\t/.+/callback_panic\.gp:22 \+0x[0-9a-f]+\ncb_sum\([^\n]*\)\n\t/.+/cbtable\.h:\d+ \+0x[0-9a-f]+\nmain\.sum\(\)\n\t/.+/callback_panic\.gp:29 \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/callback_panic\.gp:34 \+0x[0-9a-f]+\n$

// A panic that leaves a callback is fatal (step 4.11): it must not unwind through the C frames of
// cb_sum, whose state would be skipped. The program ends like an unrecovered panic: "panic: ...",
// the trace, exit code 2. The callback's own deferred calls run first; the deferred calls of the
// functions outside the C call do not run (Go would run them, and a recover there could stop the
// panic: recover comes with Phase 05).
package main

import (
	"fmt"
	"unsafe"

	"./cbtable.h"
)

func visit(user unsafe.Pointer, x C.int) C.int {
	defer fmt.Println("callback defer", int(x))
	if x == 2 {
		panic("stop at 2")
	}
	return x
}

func sum() int {
	defer fmt.Println("sum defer: not reached")
	return int(cbtable.cb_sum(5, visit, nil))
}

func main() {
	defer fmt.Println("main defer: not reached")
	fmt.Println(sum())
}

// Output:
// callback defer 0
// callback defer 1
// callback defer 2
