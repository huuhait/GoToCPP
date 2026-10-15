// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./const_from_c\.gp:10:12: x \+ 1 \(C/C\+\+ value\) is not constant$

// A constant can't be made from a C/C++ variable, as in Go and cgo.
package main

func main() {
	var x C.int = 2
	const d = x + 1
}

// Output:
