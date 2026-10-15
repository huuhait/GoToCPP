// phase: 01

// A header that the C++ compiler doesn't find is an error at its import.
// The probe can't tell anything about the names then, so the namespace
// uses stay opaque.
package main

import "<vectr>" // ERROR "cannot find header <vectr>" "header imports are not supported yet \(Phase 04\)"

func main() {
	_ = std.vector[int]{} // ERROR "C\+\+ namespaces are not supported yet \(Phase 04\)"
}
