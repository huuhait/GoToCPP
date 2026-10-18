// phase: 05
// ExitCode: 1
// Stderr: (?m)^\./not_an_interface_switch\.gp:16:14: invalid operation: v \(C/C\+\+ value\) is not an interface: a type assertion or type switch needs a Go interface value$

// A type switch on a C++ value: the same user error as for a type assertion.
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	var v std.vector[int]
	switch x := v.(type) {
	case int:
		fmt.Println("int", x)
	}
}

// Output:
