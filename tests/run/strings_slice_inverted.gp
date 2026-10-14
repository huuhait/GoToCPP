// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[3:2\]\n\ngoroutine 1 \[running\]:\n

// A low bound above the high bound panics with both.
package main

import "fmt"

func main() {
	s := "abc"
	i, j := 3, 2
	fmt.Println(s[j:i])
	fmt.Println(s[i:j])
}

// Output:
// c
