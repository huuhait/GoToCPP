// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[:9\] with length 3\n\ngoroutine 1 \[running\]:\n

// A slice expression is evaluated in its place among the calls of its statement,
// so it panics before a call to its right runs (unlike s[i], which Go checks after
// the calls).
package main

import "fmt"

func called() int {
	fmt.Println("called")
	return 1
}

func main() {
	s := "abc"
	j := 9
	fmt.Println(int(s[1]) + called())
	fmt.Println(s[:j], called())
}

// Output:
// called
// 99
