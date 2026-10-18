// phase: 04

// A C pointer has no length: it has no slice expression (cgo's rule, and Go's for a pointer
// that is not to an array). unsafe.Slice(p, n) makes the slice.
package main

import (
	"unsafe"

	"<stdlib.h>"
)

func main() {
	var p *C.int
	_ = unsafe.Slice(p, 2)
	_ = p[1:] // ERROR "cannot slice p \(variable of type \*C\.int\): a C pointer has no length; use unsafe\.Slice\(p, n\)"
}
