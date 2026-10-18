// phase: 05
// ExitCode: 2
// Stderr: ^panic: myerr 3\n\ngoroutine 1 \[running\]:\n

// A panic with an error prints its Error() text (Go's preprintpanics).
package main

import "fmt"

type MyErr struct{ Code int }

func (e MyErr) Error() string { return fmt.Sprint("myerr ", e.Code) }

func main() {
	defer fmt.Println("deferred")
	var err error = MyErr{3}
	panic(err)
}

// Output:
// deferred
