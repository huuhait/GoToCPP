// phase: 05

// A selector that go/types resolves to a Go member is the right one only when every embedded
// C/C++ type is deeper than the Go member (a member of a C++ type is one level below its field).
// At a shallower depth Go takes the C++ member if the type has the name; check can't know the
// members of a C++ type, so it says so (the gate) instead of silently taking the Go member. Naming
// the field always works; a C++ type that is deeper than the Go member does not matter.
package main

import "<vector>"

type Inner struct{ size string }
type Deep struct{ Inner }

type Shallow struct { // vector's members are at depth 1, Inner.size at depth 2
	std.vector[int]
	Deep
}

type G struct{ n int }

func (G) empty() bool { return true }

type Mid struct{ std.vector[int] }

type Deeper struct { // vector's members are at depth 2, G.empty at depth 1: Go's
	Mid
	G
}

func main() {
	var s Shallow
	_ = s.size // ERROR "selectors that a Go member and a member of an embedded C/C\+\+ type could both match \(name the field\) are not supported yet \(Phase 12\)"
	_ = s.Deep.Inner.size
	_ = s.vector.size()
	var d Deeper
	_ = d.empty()
	_ = d.n
}
