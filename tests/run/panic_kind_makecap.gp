// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: makeslice: cap out of range\n\ngoroutine 

// Uncaught panic: n := 1; _ = make([]int, 2, n). The first lines of the report and the exit code are Go's.
package main

func main() {
	n := 1
	_ = make([]int, 2, n)
}

// Output:
