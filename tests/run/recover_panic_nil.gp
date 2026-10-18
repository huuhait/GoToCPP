// phase: 05
// ExitCode: 2
// Stderr: ^panic: panic called with nil argument\n\ngoroutine 1 \[running\]:\n

// Since Go 1.21, panic(nil) panics with a *runtime.PanicNilError.
package main

import "fmt"

func main() {
	defer fmt.Println("deferred")
	panic(nil)
}

// Output:
// deferred
