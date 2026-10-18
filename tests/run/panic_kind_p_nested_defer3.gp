// phase: 05
// ExitCode: 2
// Stderr: ^panic: first\n\tpanic: second\n\tpanic: third\n\ngoroutine 

// Uncaught panic: defer func() { panic("third") }(); defer func() { panic("second") }(); panic("first"). The first lines of the report and the exit code are Go's.
package main

func main() {
	defer func() { panic("third") }()
	defer func() { panic("second") }()
	panic("first")
}

// Output:
