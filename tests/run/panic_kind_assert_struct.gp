// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is main\.St, not \*main\.St\n\ngoroutine 

// Uncaught panic: var a any = St{}; _ = a.(*St). The first lines of the report and the exit code are Go's.
package main

type St struct {
	A int
	B string
}

func main() {
	var a any = St{}
	_ = a.(*St)
}

// Output:
