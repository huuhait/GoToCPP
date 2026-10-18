// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(\*main\.St\) 0x[0-9a-f]+\n\ngoroutine 

// Uncaught panic: panic(&St{1, "b"}). The first lines of the report and the exit code are Go's.
package main

type St struct {
	A int
	B string
}

func main() {
	panic(&St{1, "b"})
}

// Output:
