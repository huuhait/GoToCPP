// phase: 04

// cgo's helpers (step 4.9): C.GoString, C.GoStringN and C.GoBytes copy C memory into gocpp
// strings and slices, with real gocpp types (s + "!" is checked as Go); C.CString copies a gocpp
// string into C's heap, and C.free frees it. Any char pointer passes (const char*, unsigned
// char*), and a nil one gives "".
package main

import (
	"fmt"

	"./cgostr.h"
)

func main() {
	s := C.GoString(cgostr.cs_greeting())
	fmt.Println(s+"!", len(s))
	fmt.Println(C.GoString(cgostr.cs_null()) == "", C.GoString(nil) == "", C.GoString(cgostr.cs_column()))
	n := C.GoStringN(cgostr.cs_bytes(), 5)
	fmt.Println(len(n), n == "ab\x00cd", C.GoStringN(nil, 0) == "")
	b := C.GoBytes(cgostr.cs_bytes(), 5)
	fmt.Println(len(b), b[2], string(b[3:]), len(C.GoBytes(nil, 0)))
	b[0] = 'X' // a copy: C's bytes stay
	fmt.Println(C.GoStringN(cgostr.cs_bytes(), 2))

	// Round trips: gocpp → C → gocpp.
	p := C.CString("round trip")
	fmt.Println(int(cgostr.cs_len(p)), C.GoString(p))
	fmt.Println(p == nil, p == p, nil != p)
	C.free(p)
	z := C.CString("a\x00b") // a NUL inside ends the C string, as in Go
	fmt.Println(int(cgostr.cs_len(z)), C.GoStringN(z, 3) == "a\x00b")
	C.free(z)
	up := cgostr.cs_upper_copy(C.cstr(s))
	fmt.Println(C.GoString(up))
	C.free(up)
	C.free(nil)

	// A C API that keeps the pointer gets a C.CString copy, freed when it is done.
	kept := C.CString(s + " (kept)")
	defer C.free(kept)
	cgostr.cs_keep(kept)
	fmt.Println(C.GoString(cgostr.cs_get_kept()))
	// A C.CString allocation may be retained by keyed/positional C pointer fields too.
	item := cgostr.CStringItem{name: kept}
	copy := cgostr.CStringItem{kept}
	fmt.Println(C.GoString(item.name), C.GoString(copy.name))
}

// Output:
// hello, C! 8
// true true column
// 5 true true
// 5 0 cd 0
// ab
// 10 round trip
// false true true
// 1 true
// HELLO, C
// hello, C (kept)
// hello, C (kept) hello, C (kept)
