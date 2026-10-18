// phase: 05
// ExitCode: 2
// Stderr: ^panic: panic called with nil argument \[recovered, repanicked\]\n\ngoroutine 1 \[running\]:\n

// Two panic(nil) are the same interface words (the PanicNilError has no size), so the chain
// prints one line.
package main

func main() {
	defer func() { panic(nil) }()
	panic(nil)
}

// Output:
