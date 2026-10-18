// phase: 04

// A C pointer converts to another pointer type only through unsafe.Pointer, as in cgo.
package main

import (
	"<cstdio>"
)

func main() {
	var p *C.char = cstdio.fgets(nil, 0, nil)
	_ = (*byte)(p) // ERROR "cannot convert p \(C pointer\) to type \*byte: convert it through unsafe.Pointer, \(\*byte\)\(unsafe.Pointer\(p\)\)"
}
