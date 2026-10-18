// phase: 05
// ExitCode: 2
// Stderr: ^panic: line one\n\tline two\n\tline three\n\ngoroutine 1 \[running\]:\n

package main

func main() {
	panic("line one\nline two\nline three")
}

// Output:
