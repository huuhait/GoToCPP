// phase: 05
// ExitCode: 2
// Stderr: ^panic: main\.MyS\("hi"\)\n\ngoroutine 

// Uncaught panic: panic(MyS("hi")). The first lines of the report and the exit code are Go's.
package main

type MyS string

func main() {
	panic(MyS("hi"))
}

// Output:
