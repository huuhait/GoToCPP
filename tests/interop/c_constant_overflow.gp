// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./c_constant_overflow\.gp:14:17: cannot use 200 \(untyped int constant\) as C\.char value in variable declaration \(overflows\)$

// An untyped constant at a C target follows Go's rules, as in cgo (step 4.9): 200 doesn't fit in
// a C.char (C++ would make it -56 silently). The other forms of the rule (C.int(2.5), a string
// constant for a *C.char, ...) have unit tests in internal/lower.
package main

import "fmt"

func main() {
	var ok C.char = 100
	var c C.char = 200
	fmt.Println(int(ok), int(c))
}

// Output:
