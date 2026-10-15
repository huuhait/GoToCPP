// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./mismatched_types\.gp:18:18: invalid operation: x \+ n \(mismatched types C\.int and int\): convert one of them: int\(x\) or C\.int\(n\)$

// As in cgo, a typed gocpp number next to a C value is an error: C++ would convert one of them
// silently. An untyped constant is fine (a C literal).
package main

import (
	"fmt"
)

func main() {
	var x C.int = 2
	n := 3
	fmt.Println(int(x + 1))
	fmt.Println(int(x) + n)
	fmt.Println(int(x + n))
}

// Output:
