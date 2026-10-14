// phase: 02
// ExitCode: 2
// Stderr: ^panic: runtime error: integer divide by zero\n\ngoroutine 1 \[running\]:\n

// Integer division by a zero variable panics with Go's runtime error.
package main

import "fmt"

func div(a, b int) int {
	return a / b
}

func main() {
	fmt.Println(div(10, 3))
	fmt.Println(div(-10, 3))
	fmt.Println(div(1, 0))
	fmt.Println("not reached")
}

// Output:
// 3
// -3
