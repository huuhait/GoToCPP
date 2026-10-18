// phase: 05
// ExitCode: 2
// Stderr: ^panic: str:x\n\ngoroutine 

// Uncaught panic: panic(Str{"x"}). The first lines of the report and the exit code are Go's.
package main

type Str struct{ s string }

func (s Str) String() string { return "str:" + s.s }

func main() {
	panic(Str{"x"})
}

// Output:
