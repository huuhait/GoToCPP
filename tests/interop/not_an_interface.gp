// phase: 05
// ExitCode: 1
// Stderr: (?m)^\./not_an_interface\.gp:17:14: invalid operation: v \(C/C\+\+ value\) is not an interface: a type assertion or type switch needs a Go interface value$

// A type assertion on a C++ value: it is not an interface, so the error is the user's (it used
// to be reported as an internal error "not supported yet").
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	var v std.vector[int]
	if s, ok := v.(fmt.Stringer); ok {
		fmt.Println(s)
	}
}

// Output:
