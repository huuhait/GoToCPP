// phase: 04

// unsafe on C values (step 4.10): calls with a C pointer or length get their Go types, an element
// of the slice that unsafe.Slice made is a C pointer again (the argv idiom: no false error), and
// the messages show what the user wrote, not the helpers that gocpp checks it with.
package main

import (
	"fmt"
	"unsafe"

	"../interop/umem.hpp"
)

type Rec struct {
	n    C.int
	vals [4]C.int
}

func main() {
	var cp **C.char
	var rec Rec
	argv := unsafe.Slice(cp, 3)
	first := unsafe.Slice(argv[0], 4)
	second := unsafe.String(argv[1:][0], 2)
	for _, a := range argv {
		_ = unsafe.Slice(a, 1)
	}
	fmt.Println(len(first), second)
	var u int = unsafe.String(argv[0], 2) // ERROR "cannot use unsafe\.String\(argv\[0\], 2\) \(value of type string\) as int value in variable declaration"
	_ = u + second                        // ERROR "mismatched types int and string"
	const k = unsafe.Sizeof(rec)          // ERROR "unsafe\.Sizeof\(rec\) \(value of type uintptr\) is not constant: the size of a C/C\+\+ type is known only to the C\+\+ compiler"
	var arr [unsafe.Sizeof(rec)]byte      // ERROR "array length unsafe\.Sizeof\(rec\) \(value of type uintptr\) must be constant"
	_ = arr
	const w = unsafe.Sizeof(umem.upoint{}) // ERROR "unsafe\.Sizeof\(.*\) \(value of type uintptr\) is not constant: the size of a C/C\+\+ type is known only to the C\+\+ compiler"
}
