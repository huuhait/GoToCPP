// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[3\] with length 3\n

// An index out of range of a slice panics with the index and the length, also in a store.
package main

import "fmt"

func main() {
	s := []int{1, 2, 3}
	for i := range s {
		s[i] *= 2
		fmt.Println(s[i])
	}
	i := len(s)
	s[i] = 1
}

// Output:
// 2
// 4
// 6
