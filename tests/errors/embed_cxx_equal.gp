// phase: 05

// A Go field and a member of an embedded C/C++ type at the same depth: Go says "ambiguous
// selector" if the C++ type has the name, and the Go field wins if it has not; check can't know.
package main

import "<vector>"

type Inner struct{ size string }

type Equal struct { // both at depth 1
	std.vector[int]
	Inner
}

func main() {
	var e Equal
	_ = e.size // ERROR "could both match"
	_ = e.Inner.size
}
