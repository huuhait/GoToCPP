// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(func\(\)\) 0x[0-9a-f]+\n\ngoroutine 

// Uncaught panic: panic(func() {}). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic(func() {})
}

// Output:
