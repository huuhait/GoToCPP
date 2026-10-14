// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[:5\] with length 3\n\ngoroutine 1 \[running\]:\n

// A high bound above the length panics with the bound and the length, before the
// low bound is checked.
package main

import "fmt"

func main() {
	s := "abc"
	i, j := 4, 5
	fmt.Println(s[1:], s[:2])
	fmt.Println(s[i:j])
}

// Output:
// bc ab
