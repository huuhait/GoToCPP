// phase: 05
// ExitCode: 2
// Stderr: ^panic: 7\n\ngoroutine 

// Uncaught panic: panic(uintptr(7)). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic(uintptr(7))
}

// Output:
