// phase: 09
// Stderr: (?s)err 1\nerr 2\nbuiltin println to stderr\n

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stdout, "out 1")
	fmt.Fprintln(os.Stderr, "err 1")
	os.Stdout.WriteString("out 2\n")
	os.Stderr.WriteString("err 2\n")
	println("builtin println to stderr")
	fmt.Println("out 3")
}

// Output:
// out 1
// out 2
// out 3
