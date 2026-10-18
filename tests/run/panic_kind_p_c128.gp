// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(\+1\.000000e\+000\+2\.000000e\+000i\)\n\ngoroutine 

// Uncaught panic: panic(complex(1, 2)). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic(complex(1, 2))
}

// Output:
