// phase: 05
// ExitCode: 2
// Stderr: (?m)^panic: errors: target cannot be nil\n

// An unrecovered errors.As(err, nil) is a Go panic with a string value.

package main

import "errors"

func main() {
	println("before")
	errors.As(errors.New("x"), nil)
	println("after")
}

// Output:
