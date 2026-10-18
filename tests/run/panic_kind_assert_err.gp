// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: int is not error: missing method Error\n\ngoroutine 

// Uncaught panic: var a any = 3; _ = a.(error). The first lines of the report and the exit code are Go's.
package main

func main() {
	var a any = 3
	_ = a.(error)
}

// Output:
