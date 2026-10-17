// phase: 04
// ExitCode: 2
// Stderr: (?m)^\./closure_cparam_error\.gp:15:7: internal error: not supported yet: func literals with C/C\+\+ parameters or results \(step 4\.11\)$

// A func literal that takes or returns a C/C++ value is a callback for C++ code: step 4.11.
// (A closure that only uses a C++ object of the function around it works: closure_members.gp.)
package main

import (
	"./objects.h"
)

func main() {
	var c objects.Counter
	f := func(d C.int) { c.add(d) }
	f(1)
}

// Output:
