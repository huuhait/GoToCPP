// phase: 02
// ExitCode: 2
// Stderr: ^panic: runtime error: integer divide by zero\n

// Modulo by zero panics too, also for the small types.
package main

import "fmt"

func main() {
	var a, b uint8 = 7, 0
	fmt.Println(a % 3)
	fmt.Println(a % b)
}

// Output:
// 1
