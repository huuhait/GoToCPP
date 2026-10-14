// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[:5\] with capacity 4\n

// A slice expression past the capacity panics with the bound and the capacity (not the
// length: s[:4] works on a slice of length 2 and capacity 4).
package main

import "fmt"

func main() {
	s := make([]int, 2, 4)
	t := s[:4]
	fmt.Println(len(t))
	j := 5
	fmt.Println(len(s[:j]))
}

// Output:
// 4
