// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: integer divide by zero\n\ngoroutine 1 \[running\]:\n

// A division by zero of a named integer type panics like one of its underlying type (step 3.9).
package main

import "fmt"

type Meters int

func perStep(total, steps Meters) Meters {
	return total / steps
}

func main() {
	fmt.Println(perStep(100, 4))
	fmt.Println(perStep(100, 0))
}

// Output:
// 25
