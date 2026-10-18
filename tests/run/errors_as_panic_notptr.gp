// phase: 05
// ExitCode: 2
// Stderr: ^panic: errors: target must be a non-nil pointer\n

// A target that is not a pointer: the panic ends the program with exit code 2.

package main

import (
	"errors"
	"fmt"
)

type E struct{}

func (E) Error() string { return "E" }

func main() {
	fmt.Println("start")
	var e E
	fmt.Println(errors.As(errors.New("x"), e))
}

// Output:
// start
