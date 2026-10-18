// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: hash of unhashable type \[\]int\n\ngoroutine 

// Uncaught panic: m := map[any]int{}; m[[]int{1}] = 1. The first lines of the report and the exit code are Go's.
package main

func main() {
	m := map[any]int{}
	m[[]int{1}] = 1
}

// Output:
