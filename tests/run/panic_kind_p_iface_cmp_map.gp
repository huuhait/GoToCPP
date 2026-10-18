// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type map\[int\]int\n\ngoroutine 

// Uncaught panic: var a, b any = map[int]int{}, map[int]int{}; _ = a == b. The first lines of the report and the exit code are Go's.
package main

func main() {
	var a, b any = map[int]int{}, map[int]int{}
	_ = a == b
}

// Output:
