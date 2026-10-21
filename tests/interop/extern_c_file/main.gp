// phase: 04

// A C function that no header declares (ADR-128): mathx.c defines it, and main.gp declares it with
// //gocpp:extern C, as Go declares a function that assembly defines. The parameters and results are
// written with C types (C.int, *C.char, C.size_t), pointers to numbers of Go and unsafe.Pointer;
// gocpp writes the declaration extern "C" int mathx_add3(int, int, int); into the generated header.
package main

import (
	"fmt"
	"unsafe"
)

//gocpp:extern C
func mathx_add3(a, b, c C.int) C.int

//gocpp:extern C
func mathx_scale(x, factor C.double) C.double

//gocpp:extern C
func mathx_length(text *C.char) C.size_t

//gocpp:extern C
func mathx_divmod(a, b C.int, quotient, remainder *int32) C.int

//gocpp:extern C
func mathx_fill(buffer unsafe.Pointer, n C.size_t)

//gocpp:extern C
func mathx_even(n uint8) bool

// add is a second Go name for the C function mathx_add3 (the directive names the C name); the two
// declarations must be spelled alike.
//
//gocpp:extern C mathx_add3
func add(a, b, c C.int) C.int

func main() {
	fmt.Println(mathx_add3(1, 2, 3), add(4, 5, 6))
	fmt.Println(mathx_scale(1.5, 4))

	text := C.CString("hello")
	fmt.Println(mathx_length(text))
	C.free(text)

	var quotient, remainder int32
	fmt.Println(mathx_divmod(17, 5, &quotient, &remainder), quotient, remainder)
	fmt.Println(mathx_divmod(1, 0, &quotient, &remainder))

	var buffer [5]byte
	mathx_fill(unsafe.Pointer(&buffer[0]), 5)
	fmt.Println(buffer)

	fmt.Println(mathx_even(4), mathx_even(7))
}

// Output:
// 6 15
// 6
// 5
// 0 3 2
// 1
// [1 2 3 4 5]
// true false
