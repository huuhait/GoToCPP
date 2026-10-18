// phase: 05
// ExitCode: 2
// Stderr: ^panic: \+2\.500000e\+000\n\ngoroutine 

// Uncaught panic: panic(float32(2.5)). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic(float32(2.5))
}

// Output:
