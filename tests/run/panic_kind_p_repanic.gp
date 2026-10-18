// phase: 05
// ExitCode: 2
// Stderr: ^panic: orig \[recovered, repanicked\]\n\ngoroutine 

// Uncaught panic: defer func() { r := recover(); panic(r) }(); panic("orig"). The first lines of the report and the exit code are Go's.
package main

func main() {
	defer func() { r := recover()
	panic(r) }()
	panic("orig")
}

// Output:
