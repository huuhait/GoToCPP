// phase: 08
// Stderr: (?m)^goroutine 1 \[running\]:$

// debug.PrintStack writes the trace to standard error.
package main

import (
	"fmt"
	"runtime/debug"
)

func main() {
	debug.PrintStack()
	fmt.Println("printed")
}

// Output:
// printed
