// phase: 02

// Dividing by a zero variable panics (exit code 2) after the output so
// far; the diff harness compares stdout and the exit code.
package main

import "fmt"

func divide(a, b int32) int32 {
	fmt.Println("divide", a, b)
	return a / b
}

func main() {
	fmt.Println(divide(7, 2))
	fmt.Println(divide(-7, 2))
	fmt.Println(divide(7, 0))
	fmt.Println("not reached")
}
