// phase: 04
// ExitCode: 2
// Stderr: ^fatal error: gocpp: a callback returned a pointer to gocpp memory to C: return a pointer from C \(malloc\), a package variable's, or the pointer that C passed\n\ngoroutine 1 \[running\]:\n

// A callback must not give C a pointer into gocpp memory (step 4.11): the last reference to the
// object is the callback's own, and C would hold a dangling address when it returns. That is a
// fatal error, where cgo's rules forbid it too.
package main

import (
	"fmt"

	"./cbtable.h"
)

func name(x C.int) *C.char {
	buf := []C.char{'h', 'i', 0}
	return &buf[0]
}

func main() {
	fmt.Println(C.GoString(cbtable.cb_name(name, 1)))
}

// Output:
