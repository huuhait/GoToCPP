// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is \[\]int, not \[\]int64\n\ngoroutine 

// Uncaught panic: var a any = []int{1}; _ = a.([]int64). The first lines of the report and the exit code are Go's.
package main

func main() {
	var a any = []int{1}
	_ = a.([]int64)
}

// Output:
