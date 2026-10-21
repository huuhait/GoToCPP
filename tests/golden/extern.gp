// phase: 04

// The C++ of extern functions (ADR-128): the header declares each one in the global namespace with
// the C++ types of its signature (raw pointers, void*, fixed-width numbers) and extern "C" or the
// namespaces of its name; a call is a call of that C++ name.
package main

import "unsafe"

//gocpp:extern C
func add3(a, b, c C.int) C.int

//gocpp:extern C
func fill(buffer unsafe.Pointer, n C.size_t, out *int32) bool

//gocpp:extern shapes::area
func area(width, height float64) float64

//gocpp:extern ::scale
func scale(text *C.char, factor C.double) *C.char

func main() {
	var out int32
	var buffer [4]byte
	println(add3(1, 2, 3), area(2, 3))
	println(fill(unsafe.Pointer(&buffer[0]), 4, &out))
	println(scale(nil, 1.5) == nil)
}
