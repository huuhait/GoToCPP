// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./not_a_type\.gp:11:10: std\.cout is a C/C\+\+ value, not a type$

// A C++ value (std::cout, an object) where a type must be.
package main

import "<iostream>"

func main() {
	var out std.cout
	_ = out
}

// Output:
