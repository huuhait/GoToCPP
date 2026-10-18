// phase: 05
// ExitCode: 2
// Stderr: ^panic: panic called with nil argument\n\ngoroutine 

// Uncaught panic: var e error; panic(e). The first lines of the report and the exit code are Go's.
package main

func main() {
	var e error
	panic(e)
}

// Output:
