// phase: 05
// ExitCode: 2
// Stderr: ^panic:\ runtime\ error:\ comparing\ uncomparable\ type\ \[\]int\n\ngoroutine 

// Uncaught panic: var a, b any = []int{1}, []int{1}; _ = a == b. The first lines of the report and the exit code are Go's.
package main

func main() {
	var a, b any = []int{1}, []int{1}
	_ = a == b
}

// Output:
