// phase: 05
// ExitCode: 2
// Stderr: ^panic: fatal\n\ngoroutine 1 \[running\]:\n

// An unrecovered panic runs the deferred calls of every frame, then exits with code 2; output of the
// deferred calls comes before the panic message.
package main

import "fmt"

func inner() {
	defer fmt.Println("inner deferred")
	panic("fatal")
}

func main() {
	defer fmt.Println("main deferred")
	inner()
	fmt.Println("not reached")
}

// Output:
// inner deferred
// main deferred
