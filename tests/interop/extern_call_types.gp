// phase: 04

// The types of the arguments of an extern function are checked against the declaration when the
// program is built (ADR-128), at the .gp line of the call. Their number is checked earlier
// (tests/errors/extern_call_arguments.gp).
package main

//gocpp:extern C
func add3(a, b, c C.int) C.int

func main() {
	println(add3(1, 2, 3))
	println(add3("a", 2, 3)) // ERROR "cannot use \x22a\x22 \(untyped string constant\) as C.int value in argument to add3"
}
