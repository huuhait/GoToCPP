// phase: 05
// ExitCode: 2
// Stderr: ^panic: first\n\tpanic: myerr 2\n\ngoroutine 

// Uncaught panic: defer func() { panic(&MyErr{2}) }(); panic(errors.New("first")). The first lines of the report and the exit code are Go's.
package main

import (
	"errors"
	"fmt"
)

type MyErr struct{ Code int }

func (e *MyErr) Error() string { return fmt.Sprintf("myerr %d", e.Code) }

func main() {
	defer func() { panic(&MyErr{2}) }()
	panic(errors.New("first"))
}

// Output:
