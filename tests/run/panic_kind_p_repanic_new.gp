// phase: 05
// ExitCode: 2
// Stderr: ^panic: orig \[recovered\]\n\tpanic: new\n\ngoroutine 

// Uncaught panic: defer func() { recover(); panic("new") }(); panic("orig"). The first lines of the report and the exit code are Go's.
package main

func main() {
	defer func() { recover()
	panic("new") }()
	panic("orig")
}

// Output:
