// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(main\.PStr\) 0x[0-9a-f]+\n\ngoroutine 

// Uncaught panic: panic(PStr{"x"}). The first lines of the report and the exit code are Go's.
package main

type PStr struct{ s string }

func (s *PStr) String() string { return "pstr:" + s.s }

func main() {
	panic(PStr{"x"})
}

// Output:
