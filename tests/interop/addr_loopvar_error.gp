// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./addr_loopvar_error\.gp:16:20: & of the variable of a 3-clause for loop \(i\): Go gives each iteration its own variable, and a C/C\+\+ address can't keep it alive: copy it into a variable of the body, j := i, and take &j$

// &i of the variable of a 3-clause for loop is an error: each iteration has its own i in Go, which
// a pointer can keep, and a C++ address can't. (The variable of a range is a new one for each
// iteration in C++ too.)
package main

import (
	"./objects.h"
)

func main() {
	for i := C.int(0); i < 3; i++ {
		objects.bump_ptr(&i)
	}
}

// Output:
