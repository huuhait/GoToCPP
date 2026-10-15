// phase: 03
// ExitCode: 2
// Stderr: ^panic: value method weak\.Pointer\[\.\.\.\]\.Value called using nil \*Pointer\[\.\.\.\] pointer\n\ngoroutine 1 \[running\]:\n

// The method expression (*weak.Pointer[T]).Value called with a nil pointer panics in Go's
// wrapper, whose message writes the type arguments of the instantiated type as [...].
package main

import (
	"fmt"
	"weak"
)

type T struct{ n int }

func main() {
	value := (*weak.Pointer[T]).Value
	var z weak.Pointer[T]
	fmt.Println(value(&z) == nil)
	var np *weak.Pointer[T]
	fmt.Println(value(np))
}

// Output:
// true
