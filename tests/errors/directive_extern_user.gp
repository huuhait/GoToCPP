// phase: 04

// Calls of a function that a header declares need no //gocpp:extern; a call of an extern function
// type-checks like any call, and its arguments are C values (ADR-128).

package main

//gocpp:extern C
func add3(a, b, c C.int) C.int

//gocpp:extern C
func scale(p *C.char, factor C.double) *C.char

func main() {
	var n C.int = add3(1, 2, 3)
	s := C.CString("x")
	_ = scale(s, 2.5)
	println(n)
}
