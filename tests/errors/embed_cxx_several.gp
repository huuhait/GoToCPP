// phase: 05

// A struct that embeds one C/C++ type gets its members promoted (tests/interop/embed_cxx.gp). With
// several, check can't tell which type has a member: the gate says so, at each use, and naming the
// field works.
package main

import (
	"<string>"
	"<vector>"
)

type Both struct {
	std.vector[int]
	std.string
}

func main() {
	var b Both
	b.vector.push_back(1)
	b.string.push_back('x')
	b.push_back(1)   // ERROR "promoted fields and methods of several embedded C/C\+\+ types are not supported yet \(Phase 12\)"
	_ = b.size()     // ERROR "promoted fields and methods of several embedded C/C\+\+ types are not supported yet \(Phase 12\)"
	_ = b.vector.size()
}
