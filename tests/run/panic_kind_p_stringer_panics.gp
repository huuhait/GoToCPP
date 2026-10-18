// phase: 05
// ExitCode: 2
// Stderr: ^fatal error: panic while printing panic value: in String\n\ngoroutine 

// Uncaught panic: panic(StrP{}). The first lines of the report and the exit code are Go's.
package main

type StrP struct{}

func (StrP) String() string { panic("in String") }

func main() {
	panic(StrP{})
}

// Output:
