// phase: 05

// A Go interface type as the argument of a C++ template waits for typed interop (Phase 12): the
// error is the user's ("not available yet"), not an internal error, and names that phase.
package main

import "<vector>"

type Shape interface{ Area() int }

func main() {
	var w std.vector[Shape] // ERROR "^interface types as C\+\+ template arguments are not available yet \(typed interop: Phase 12\)"
	_ = w
}
