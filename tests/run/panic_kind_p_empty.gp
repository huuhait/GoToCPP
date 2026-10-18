// phase: 05
// ExitCode: 2
// Stderr: ^panic: \n\ngoroutine 

// Uncaught panic: panic(""). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic("")
}

// Output:
