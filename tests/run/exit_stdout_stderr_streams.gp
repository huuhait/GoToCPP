// phase: 08
// Stderr: (?s)^to stderr 1\nto stderr 2\n$

// stdout and stderr are separate streams that are both complete at the end: an exit with live
// goroutines loses neither.
package main

import (
	"fmt"
	"os"
)

func main() {
	go func() {
		select {}
	}()
	fmt.Println("to stdout 1")
	fmt.Fprintln(os.Stderr, "to stderr 1")
	fmt.Println("to stdout 2")
	fmt.Fprintln(os.Stderr, "to stderr 2")
}

// Output:
// to stdout 1
// to stdout 2
