// phase: 05
// ExitCode: 2
// Stderr: ^panic: %!w\(<nil>\)\n\ngoroutine 

// Uncaught panic: panic(fmt.Errorf("%w", nil)). The first lines of the report and the exit code are Go's.
package main

import "fmt"

func main() {
	panic(fmt.Errorf("%w", nil))
}

// Output:
