// phase: 04

// What cannot be a C function pointer is an error at the .gp line (step 4.11): a closure that
// captures variables, a func value that is only known at run time, a func with the wrong number
// of parameters or the wrong result, and a variadic callback. The message of the helper says what
// to write instead.
package main

import (
	"unsafe"

	"./cbtable.h"
	"<stdlib.h>"
)

func visit(user unsafe.Pointer, i C.int) C.int { return i }
func visit1(i C.int) C.int                     { return i }
func nothing(user unsafe.Pointer, i C.int)     {}

func byPtr(a, b *C.int) C.int { return 0 }

func main() {
	n := 5
	cbtable.cb_sum(3, func(user unsafe.Pointer, i C.int) C.int { return i + C.int(n) }, nil) // ERROR "C\+\+: a func with captured variables, or a func value that is only known at run time, can't be a C function pointer"
	f := func(x C.int) {}
	cbtable.cb_register(f)          // ERROR "C\+\+: a func with captured variables, or a func value that is only known at run time, can't be a C function pointer"
	cbtable.cb_sum(3, visit1, nil)  // ERROR "C\+\+: this func has a different number of parameters than the C function pointer type it is passed as"
	cbtable.cb_logger(visit1)       // ERROR "C\+\+: a callback can't be a variadic C function pointer"
	cbtable.cb_vcall(3, visit)      // ERROR "C\+\+: (passing objects of non-trivially-copyable type|cannot pass object of non-trivial type)"
	stdlib.qsort(nil, 0, 4, byPtr)  // ERROR "C\+\+: the C parameter is a void\*: declare this parameter as unsafe.Pointer"
	cbtable.cb_sum(3, nothing, nil) // ERROR "C\+\+: the C function pointer type and this func disagree about a result"
}
