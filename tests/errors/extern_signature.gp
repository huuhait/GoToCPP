// phase: 04

// The parameters and results of an extern function (//gocpp:extern, ADR-128) have one C++
// spelling: a C or C++ type, unsafe.Pointer, or a number or bool of Go, and pointers to them.

package main

import (
	"fmt"
	"unsafe"
)

type Handle int

//gocpp:extern C
func good(p *C.char, n C.int, q unsafe.Pointer, flag bool, u uint8, x float64, out *int32) C.size_t

//gocpp:extern C
func str(s string) C.int // ERROR "extern function str: parameter type string is a Go type that C\+\+ does not have"

//gocpp:extern C
func slice(xs []byte) C.int // ERROR "extern function slice: parameter type \[\]byte is a Go type that C\+\+ does not have"

//gocpp:extern C
func result() string // ERROR "extern function result: result type string is a Go type that C\+\+ does not have"

//gocpp:extern C
func named(h Handle) C.int // ERROR "extern function named: parameter type Handle is a type declared in Go"

//gocpp:extern C
func std(d fmt.Stringer) C.int // ERROR "extern function std: parameter type fmt.Stringer is a type of the Go package fmt"

//gocpp:extern C
func callback(f func(C.int) C.int) // ERROR "extern function callback: parameter type func\(C.int\) C.int is a Go type that C\+\+ does not have"

//gocpp:extern C
func variadic(format *C.char, args ...any) C.int // ERROR "extern function variadic can not be variadic: C varargs are not supported"

//gocpp:extern C
func two() (C.int, C.int) // ERROR "extern function two returns more than one value"

//gocpp:extern C
func iface(e error) // ERROR "extern function iface: parameter type error is a Go type that C\+\+ does not have"

//gocpp:extern C
func goInt(n int) C.int // ERROR "extern function goInt: parameter type int is the Go int \(64 bits\), which is not the C int \(32 bits\): write C.int, int32, int64"

//gocpp:extern C
func goUint(n uint) // ERROR "extern function goUint: parameter type uint is the Go uint \(64 bits\)"

//gocpp:extern C
func goIntResult() int // ERROR "extern function goIntResult: result type int is the Go int"

//gocpp:extern C
func voidResult() unsafe.Pointer // ERROR "extern function voidResult: result type unsafe.Pointer can not be the result of a C function: a gocpp pointer is a managed object"

//gocpp:extern C
func numberPointerResult() *int32 // ERROR "extern function numberPointerResult: result type \*int32 can not be the result of a C function"

func main() { var _ fmt.Stringer }
