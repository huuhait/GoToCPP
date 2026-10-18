// phase: 04

// unsafe and pointer conversions of C values that only the C++ compiler can tell are wrong
// (step 4.10), at their .gp lines: unsafe.Slice of a void* (no element type), a C pointer
// converted to a gocpp pointer other than through unsafe.Pointer (gocpp knows nothing of the
// type of a call's result), and a slice expression of a C++ container (only a C array has one).
package main

import (
	"unsafe"

	"<cstdlib>"
	"<cstring>"
	"<vector>"
)

func main() {
	raw := cstdlib.malloc(8)
	_ = unsafe.Slice(raw, 2) // ERROR "C\+\+: unsafe.Slice needs a pointer to an element type, not void\*"
	_ = (*byte)(cstring.strchr(C.cstr("abc"), 'b')) // ERROR "C\+\+: a C pointer converts to a gocpp pointer only through unsafe.Pointer"
	v := std.vector[int]{1, 2, 3}
	_ = v[1:] // ERROR "C\+\+: only a C array or a std::array has a slice expression"
	_ = (*byte)(raw)
	_ = unsafe.Slice((*byte)(raw), 2)
	cstdlib.free(raw)
}
