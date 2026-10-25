// phase: 08
// ExitCode: 2
// Stderr: ^panic: x\n\tpanic: outer\n\ngoroutine 1 \[running\]:\n

// The same in the main goroutine: Goexit during a panic, then a deferred call that panics.
package main

import "runtime"

func main() {
	defer func() { panic("outer") }()
	defer runtime.Goexit()
	panic("x")
}

// Output:
