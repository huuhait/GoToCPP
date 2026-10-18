// phase: 05
// ExitCode: 2
// Stderr: ^panic: outer\n\tpanic: inner1\n\tpanic: inner2\n\ngoroutine 

// Uncaught panic: defer func() { defer func() { panic("inner2") }(); panic("inner1") }(); panic("outer"). The first lines of the report and the exit code are Go's.
package main

func main() {
	defer func() { defer func() { panic("inner2") }()
	panic("inner1") }()
	panic("outer")
}

// Output:
