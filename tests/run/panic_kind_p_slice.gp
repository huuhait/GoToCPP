// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(\[\]int\) 0x[0-9a-f]+\n\ngoroutine 

// Uncaught panic: panic([]int{1,2}). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic([]int{1,2})
}

// Output:
