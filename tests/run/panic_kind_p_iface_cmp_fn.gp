// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type func\(\)\n\ngoroutine 

// Uncaught panic: var a, b any = func() {}, func() {}; _ = a == b. The first lines of the report and the exit code are Go's.
package main

func main() {
	var a, b any = func() {}, func() {}
	_ = a == b
}

// Output:
