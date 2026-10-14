// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: slice bounds out of range \[18446744073709551615:3\]\n\ngoroutine 1 \[running\]:\n

// s[i:] with i above the length panics with i and the length; an unsigned bound
// prints as unsigned.
package main

import "fmt"

func main() {
	s := "abc"
	var i uint64 = 3
	fmt.Println(s[i:] == "")
	i--
	i -= 3
	fmt.Println(s[i:])
}

// Output:
// true
