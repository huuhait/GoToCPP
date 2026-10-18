// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type main\.S\n\ngoroutine 

// Uncaught panic: type S struct{ f []int }; var a, b any = S{}, S{}; _ = a == b. The first lines of the report and the exit code are Go's.
package main

func main() {
	type S struct{ f []int }
	var a, b any = S{}, S{}
	_ = a == b
}

// Output:
