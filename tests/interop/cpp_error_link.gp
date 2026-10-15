// phase: 04

// A C function that a header declares but nothing defines (a missing .c file or library) is an
// error of the program at the .gp line that calls it, with a hint.
package main

import "./undefined_fn.h"

func main() {
	undefined_fn.gocpp_test_undefined_fn(1) // ERROR "C\+\+: undefined reference to 'gocpp_test_undefined_fn'"
}
