// phase: 01

// gocpp/cgoh is the hidden package of the conversions of opaque values
// (Phase 01 step 1.10b): user code can't import it.
package main

import gp_cgoh "gocpp/cgoh" // ERROR "use of internal package gocpp/cgoh not allowed"

func main() {
	_ = gp_cgoh.As[int]
}
