// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[-1:\]\n\ngoroutine 1 \[running\]:\n

// A negative low bound panics without the high bound, like Go.
package main

import "fmt"

func main() {
	s := "abc"
	i := -1
	fmt.Println(s[i+1:])
	fmt.Println(s[i:])
}

// Output:
// abc
