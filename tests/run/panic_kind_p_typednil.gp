// phase: 05
// ExitCode: 2
// Stderr: ^fatal error: panic while printing panic value: type runtime\.errorString\n

// Uncaught panic: panic of a nil *MyErr whose Error method dereferences it while the panic is printed, so
// Go reports "panic while printing panic value" (followed by a [signal SIGSEGV ...] line that gocpp does not
// print: the regexp stops at the first line).
package main

import "fmt"

type MyErr struct{ Code int }

func (e *MyErr) Error() string { return fmt.Sprintf("myerr %d", e.Code) }

func main() {
	var e *MyErr
	panic(e)
}

// Output:
