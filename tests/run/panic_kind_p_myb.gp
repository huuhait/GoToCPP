// phase: 05
// ExitCode: 2
// Stderr: ^panic: main\.MyB\(true\)\n\ngoroutine 

// Uncaught panic: panic(MyB(true)). The first lines of the report and the exit code are Go's.
package main

type MyB bool

func main() {
	panic(MyB(true))
}

// Output:
