// phase: 05
// ExitCode: 2
// Stderr: ^panic: verr 4\n\ngoroutine 

// Uncaught panic: panic(VErr{4}). The first lines of the report and the exit code are Go's.
package main

import "fmt"

type VErr struct{ Code int }

func (e VErr) Error() string { return fmt.Sprintf("verr %d", e.Code) }

func main() {
	panic(VErr{4})
}

// Output:
