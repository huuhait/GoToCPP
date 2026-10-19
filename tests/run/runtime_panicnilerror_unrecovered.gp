// phase: 08
// ExitCode: 2
// Stderr: ^panic: panic called with nil argument( \[recovered\])?\n\ngoroutine 1 \[running\]:\n

// An unrecovered panic(nil) ends the program with exit code 2 and Go's text.
package main

import "runtime"

func main() {
	var e runtime.Error
	_ = e
	panic(nil)
}

// Output:
