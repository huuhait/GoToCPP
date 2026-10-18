// phase: 05
// ExitCode: 2
// Stderr: ^panic: 200\n\ngoroutine 

// Uncaught panic: panic(uint8(200)). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic(uint8(200))
}

// Output:
