// phase: 04
// ExitCode: 2
// Stderr: (?m)^panic: runtime error: invalid memory address or nil pointer dereference$

// defer s.c.inc() with a nil gocpp pointer s panics when the defer statement runs, as in Go (the
// call's object is evaluated there), not when the deferred call does, after the body's output.
package main

import (
	"fmt"

	"./objects.h"
)

type S struct {
	c objects.Counter
}

func main() {
	var s *S
	defer fmt.Println("deferred")
	defer s.c.inc()
	fmt.Println("not reached")
}

// Output:
// deferred
