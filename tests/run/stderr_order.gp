// phase: 02
// Stderr: (?s)^to stderr 1\nto stderr 2 42 true\n$

// println writes to stderr, fmt to stdout. The run harness checks each
// stream on its own.
package main

import "fmt"

func main() {
	fmt.Println("to stdout 1")
	println("to stderr 1")
	fmt.Print("to stdout ", 2, "\n")
	println("to stderr 2", 42, true)
	fmt.Println("to stdout 3")
}

// Output:
// to stdout 1
// to stdout 2
// to stdout 3
