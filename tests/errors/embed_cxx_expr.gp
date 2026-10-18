// phase: 05

// A method expression of a member that a struct promotes from its one embedded C/C++ type has no
// C++ form yet: the gate says so (the members themselves are promoted, tests/interop/embed_cxx.gp).
package main

import "<vector>"

type MV struct {
	std.vector[int]
	n int
}

func main() {
	f := MV.size            // ERROR "method expressions of promoted members of an embedded C/C\+\+ type are not supported yet \(Phase 12\)"
	g := (*MV).push_back    // ERROR "method expressions of promoted members of an embedded C/C\+\+ type are not supported yet \(Phase 12\)"
	var mv MV
	mv.push_back(1)
	_, _ = f, g
}
