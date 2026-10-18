// phase: 05
// ExitCode: 2
// Stderr: ^panic: a\n\tb\n\ngoroutine 

// Uncaught panic: panic(errors.Join(errors.New("a"), errors.New("b"))). The first lines of the report and the exit code are Go's.
package main

import "errors"

func main() {
	panic(errors.Join(errors.New("a"), errors.New("b")))
}

// Output:
