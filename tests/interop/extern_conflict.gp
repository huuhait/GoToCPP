// phase: 04

// Two declarations of one C function must agree (ADR-128): C would not tell, and the program
// would pass its arguments in the wrong registers.
package main

//gocpp:extern C
func f(x C.int) C.int

//gocpp:extern C f
func g(x C.long) C.int // ERROR "extern function f is declared with the C signature int f\(long\), and elsewhere with another one, int f\(int\); the other declaration is at .*extern_conflict\.gp:8:6"

func main() {
	println(f(1), g(2))
}
