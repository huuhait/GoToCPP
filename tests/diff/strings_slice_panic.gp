// phase: 03

// A substring out of range panics (exit code 2) after the output before it.
package main

import "fmt"

func main() {
	s := "abc"
	i, j := 1, 5
	fmt.Println(s[i:3], s[:i])
	fmt.Println(s[i:j])
}
