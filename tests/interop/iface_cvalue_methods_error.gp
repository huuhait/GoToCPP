// phase: 05

// A C++ value in a Go interface that has methods waits for typed interop (Phase 12): the error is
// the user's ("not available yet"), not an internal error.
package main

import "<vector>"

type Sizer interface{ Size() int }

func main() {
	v := std.vector[int]{1, 2}
	var s Sizer = v // ERROR "^a C/C\+\+ value in an interface with methods \(main\.Sizer\) is not available yet"
	_ = s
}
