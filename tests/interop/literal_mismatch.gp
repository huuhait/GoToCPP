// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./literal_mismatch\.gp:16:17: cannot use n \(variable of type int\) as C\.int value in array or slice literal: convert it, C\.int\(n\)$

// As in cgo, a typed gocpp number in a gocpp literal whose elements are C values is an error
// (step 4.6): C++ would convert it silently. An untyped constant is fine (a C literal), and so is
// a converted value.
package main

import "fmt"

func main() {
	n := 3
	ok := []C.int{1, C.int(n)}
	fmt.Println(len(ok))
	bad := []C.int{n}
	fmt.Println(len(bad))
}

// Output:
