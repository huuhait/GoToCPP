// phase: 04

// The cgo helpers have cgo's declarations (step 4.9), so their results are typed (s is a string)
// and a gocpp argument of another type is Go's error, with the names as the user wrote them. A C
// value passes silently: C.GoString takes any char pointer.
package main

import (
	"fmt"
)

func main() {
	var p *C.uchar
	s := C.GoString(p)
	var n int = s + "!"     // ERROR "cannot use s \+ \x22!\x22 \(value of type string\) as int value in variable declaration"
	_ = s + 1               // ERROR "invalid operation: s \+ 1 \(mismatched types string and untyped int\)"
	var b []byte
	_ = C.GoString(b)       // ERROR "cannot use b \(variable of type \[\]byte\) as \*C\.char value in argument to C\.GoString"
	k := 3
	_ = C.GoStringN(nil, k) // ERROR "cannot use k \(variable of type int\) as C\.int value in argument to C\.GoStringN"
	_ = C.GoBytes(&k, 1)    // ERROR "cannot use &k \(value of type \*int\) as unsafe\.Pointer value in argument to C\.GoBytes"
	_ = C.CString(k)        // ERROR "cannot use k \(variable of type int\) as string value in argument to C\.CString"
	C.free(&k)              // ERROR "cannot use &k \(value of type \*int\) as unsafe\.Pointer value in argument to C\.free"
	_ = C.cstr(b)           // ERROR "cannot use b \(variable of type \[\]byte\) as string value in argument to C\.cstr"
	var bs []byte = C.GoBytes(p, 2)
	var x string = C.GoBytes(p, 2) // ERROR "cannot use C\.GoBytes\(p, 2\) \(value of type \[\]byte\) as string value in variable declaration"
	_ = C.GoString()               // ERROR "not enough arguments in call to C\.GoString\n\thave \(\)\n\twant \(\*C\.char\)"
	fmt.Println(n, bs, x)
}
