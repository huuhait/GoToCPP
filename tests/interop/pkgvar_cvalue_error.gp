// phase: 04

// A package variable without a type whose value is a C/C++ value (a call of a C++ function) has no type that gocpp can write in the
// package's header: the build says so at the variable, as a limit with the remedy, not as an internal error.
package main

import (
	"fmt"

	"<string>"
)

var s = std.to_string(5) // ERROR "package variables of C/C\+\+ values without a type are not supported yet: write the type of s \(var s T = ...\)$"

func main() {
	fmt.Println(int(s.size()))
}
