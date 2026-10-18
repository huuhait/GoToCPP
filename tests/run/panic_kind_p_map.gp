// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(map\[string\]int\) 0x[0-9a-f]+\n\ngoroutine 

// Uncaught panic: panic(map[string]int{"a": 1}). The first lines of the report and the exit code are Go's.
package main

func main() {
	panic(map[string]int{"a": 1})
}

// Output:
