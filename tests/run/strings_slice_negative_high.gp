// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[:-2\]\n\ngoroutine 1 \[running\]:\n

// A negative high bound panics without the length.
package main

import "fmt"

func main() {
	s := "abc"
	j := -2
	fmt.Println(s[:j+2] == "")
	fmt.Println(s[:j])
}

// Output:
// true
