// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: error is \*errors\.errorString, not \*main\.MyErr\n

// The dynamic type of errors.New is *errors.errorString, as Go's assertion panic names it.

package main

import (
	"errors"
	"fmt"
)

type MyErr struct{}

func (*MyErr) Error() string { return "my" }

func main() {
	err := errors.New("boom")
	fmt.Println("asserting")
	fmt.Println(err.(*MyErr))
}

// Output:
// asserting
