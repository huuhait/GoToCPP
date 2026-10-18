// phase: 05
// ExitCode: 2
// Stderr: ^panic: main\.MyI\(3\)\n\ngoroutine 

// Uncaught panic: panic(MyI(3)). The first lines of the report and the exit code are Go's.
package main

type MyI int

func main() {
	panic(MyI(3))
}

// Output:
