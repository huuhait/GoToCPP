// phase: 05

// A Go method and a member of an embedded C/C++ type at the same depth: see embed_cxx_equal.gp.
package main

import "<vector>"

type G struct{ n int }

func (G) empty() bool { return true }

type EqualMethod struct {
	std.vector[int]
	G
}

func main() {
	var m EqualMethod
	_ = m.empty() // ERROR "could both match"
	_ = m.G.empty()
}
