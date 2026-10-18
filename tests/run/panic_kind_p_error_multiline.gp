// phase: 05
// ExitCode: 2
// Stderr: ^panic: l1\n\tl2\n\ngoroutine 

// Uncaught panic: panic(errors.New("l1\nl2")). The first lines of the report and the exit code are Go's.
package main

import "errors"

func main() {
	panic(errors.New("l1\nl2"))
}

// Output:
