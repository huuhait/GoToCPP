// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface is nil, not main\.I\n\ngoroutine 

// Uncaught panic: var a any; _ = a.(I). The first lines of the report and the exit code are Go's.
package main

type I interface{ M() }

func main() {
	var a any
	_ = a.(I)
}

// Output:
