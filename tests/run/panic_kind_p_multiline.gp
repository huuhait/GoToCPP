// phase: 05
// ExitCode: 2
// Stderr: ^panic: a\n\tb\n\ngoroutine 

// Uncaught panic: panic("a\nb"). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic("a\nb")
}

// Output:
