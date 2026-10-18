// phase: 05
// ExitCode: 2
// Stderr: ^panic: after\n\ngoroutine 

// Uncaught panic: defer func() { panic("after") }(); func() { defer func() { recover() }(); panic("inner") }(). The first lines of the report and the exit code are Go's.
package main

func main() {
	defer func() { panic("after") }()
	func() { defer func() { recover() }()
	panic("inner") }()
}

// Output:
