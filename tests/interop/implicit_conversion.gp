// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./implicit_conversion\.gp:13:15: cannot use cmath\.pow\(10, 30\) \(C/C\+\+ value\) as int value in variable declaration: convert it, int\(cmath\.pow\(10, 30\)\)$

// A C/C++ value doesn't convert implicitly to a gocpp type, as in cgo: C++'s implicit
// conversion of a double out of an int's range would be undefined behavior. int(x) converts it
// with Go's rules (float_conversions.gp).
package main

import "<cmath>"

func main() {
	var iv int = cmath.pow(10, 30)
	_ = iv
}

// Output:
