// phase: 05
// ExitCode: 2
// Stderr: (?m)^panic: boom$
// Stderr: (?m)^before$

// panic(err) with an error from errors.New: the panic prints the error's text.

package main

import "errors"

func main() {
	println("before")
	panic(errors.New("boom"))
}

// Output:
