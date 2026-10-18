// phase: 05
// ExitCode: 2
// Stderr: ^panic: x \[recovered\]\n\tpanic: panic called with nil argument\n\ngoroutine 

// Uncaught panic: defer func() { recover(); panic(nil) }(); panic("x"). The first lines of the report and the exit code are Go's.
package main

func main() {
	defer func() { recover()
	panic(nil) }()
	panic("x")
}

// Output:
