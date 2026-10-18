// phase: 05
// ExitCode: 2
// Stderr: ^panic: main\.MyF\(\+2\.500000e\+000\)\n\ngoroutine 

// Uncaught panic: panic(MyF(2.5)). The first lines of the report and the exit code are Go's.
package main

type MyF float64

func main() {
	panic(MyF(2.5))
}

// Output:
