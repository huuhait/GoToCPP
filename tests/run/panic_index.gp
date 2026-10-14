// phase: 02
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[5\] with length 3\n

// Indexing a string out of range panics with the index and the length.
package main

import "fmt"

func main() {
	s := "abc"
	for i := range 3 {
		fmt.Println(s[i])
	}
	i := 5
	fmt.Println(s[i])
}

// Output:
// 97
// 98
// 99
