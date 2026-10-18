// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: int is not main\.I: missing method M\n\ngoroutine 

// Uncaught panic: var a any = 1; _ = a.(I). The first lines of the report and the exit code are Go's.
package main

type I interface{ M() }

func main() {
	var a any = 1
	_ = a.(I)
}

// Output:
