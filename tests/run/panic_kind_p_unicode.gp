// phase: 05
// ExitCode: 2
// Stderr: ^panic: héllo ☺\n\ngoroutine 

// Uncaught panic: panic("héllo ☺"). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic("héllo ☺")
}

// Output:
