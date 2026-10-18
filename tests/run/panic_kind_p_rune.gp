// phase: 05
// ExitCode: 2
// Stderr: ^panic: 120\n\ngoroutine 

// Uncaught panic: panic('x'). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic('x')
}

// Output:
