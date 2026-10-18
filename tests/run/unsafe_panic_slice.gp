// phase: 04
// ExitCode: 2
// Stderr: ^panic: runtime error: unsafe.Slice: len out of range\n\ngoroutine 1 \[running\]:\n

// unsafe.Slice panics for a negative length, as Go does (a runtime error, which defer and
// recover see before the program ends).
package main

import (
	"fmt"
	"unsafe"
)

func main() {
	xs := []int{1, 2, 3}
	n := 2
	fmt.Println(len(unsafe.Slice(&xs[0], n)))
	n = -1
	fmt.Println(len(unsafe.Slice(&xs[0], n)))
}

// Output:
// 2
