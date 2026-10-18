// phase: 04
// ExitCode: 2
// Stderr: ^panic: runtime error: unsafe.String: ptr is nil and len is not zero\n\ngoroutine 1 \[running\]:\n

// unsafe.String of a nil pointer with a length above 0 panics, as in Go.
package main

import (
	"fmt"
	"unsafe"
)

func main() {
	var p *byte
	fmt.Println(unsafe.String(p, 0) == "")
	n := 3
	fmt.Println(unsafe.String(p, n))
}

// Output:
// true
