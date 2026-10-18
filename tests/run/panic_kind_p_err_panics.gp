// phase: 05
// ExitCode: 2
// Stderr: ^fatal error: panic while printing panic value: in Error\n\ngoroutine 

// Uncaught panic: panic(ErrP{}). The first lines of the report and the exit code are Go's.
package main

type ErrP struct{}

func (ErrP) Error() string { panic("in Error") }

func main() {
	panic(ErrP{})
}

// Output:
