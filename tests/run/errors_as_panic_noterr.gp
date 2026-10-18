// phase: 05
// ExitCode: 2
// Stderr: ^panic: errors: \*target must be interface or implement error\n

// A pointer to a type that is neither an interface nor an error.

package main

import (
	"errors"
	"fmt"
)

func main() {
	defer fmt.Println("deferred runs")
	var n int
	fmt.Println(errors.As(errors.New("x"), &n))
}

// Output:
// deferred runs
