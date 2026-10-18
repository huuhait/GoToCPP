// phase: 05
// ExitCode: 2
// Stderr: ^panic: wrap: inner\n\ngoroutine 

// Uncaught panic: panic(fmt.Errorf("wrap: %w", errors.New("inner"))). The first lines of the report and the exit code are Go's.
package main

import (
	"errors"
	"fmt"
)

func main() {
	panic(fmt.Errorf("wrap: %w", errors.New("inner")))
}

// Output:
