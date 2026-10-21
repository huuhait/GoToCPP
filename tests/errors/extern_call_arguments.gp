// phase: 04

// The arguments of a call of an extern function are checked against the declaration like any call's
// (ADR-128): their number. (Their types are checked when the program is built: tests/interop/extern_call_types.gp.)

package main

//gocpp:extern C
func add3(a, b, c C.int) C.int

func main() {
	println(add3(1, 2, 3))
	println(add3(1, 2)) // ERROR "not enough arguments in call to add3"
	println(add3(1, 2, 3, 4)) // ERROR "too many arguments in call to add3"
}
