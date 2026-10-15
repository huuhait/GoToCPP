// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./assign_to_call\.gp:15:2: cannot assign to v\.at\(0\) \(neither addressable nor a map index expression\)$

// A call's result is no place in Go, also where C++ returns a reference: v[0] = 5 works, v.at(0)
// = 5 doesn't.
package main

import "<vector>"

func main() {
	var v std.vector[int]
	v.push_back(1)
	v[0] = 5
	v.at(0) = 5
}

// Output:
