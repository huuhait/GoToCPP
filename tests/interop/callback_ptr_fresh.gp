// phase: 04
// ExitCode: 2
// Stderr: ^fatal error: gocpp: a callback returned a pointer to gocpp memory to C: return a pointer from C \(malloc\), a package variable's, or the pointer that C passed\n\ngoroutine 1 \[running\]:\n

// Handing back the pointer that C passed is fine (callbacks_values.gp), but a pointer to a struct
// that the callback made itself is not: the callback's reference is the only one, and C would hold
// a dangling address (step 4.11).
package main

import (
	"fmt"
	"unsafe"

	"./cbtable.h"
)

func fresh(u unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(&cbtable.cb_ops{})
}

func main() {
	keep := &cbtable.cb_ops{} // a registered struct is alive: the check can't just look at the registry
	fmt.Println(cbtable.cb_ud(fresh, unsafe.Pointer(keep)) != nil)
}

// Output:
