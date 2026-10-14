// phase: 02
// ExitCode: 2
// Stderr: ^panic: -42\n

// panic with an integer value prints the number, the way print does.
package main

import "fmt"

func main() {
	fmt.Println("before")
	n := -42
	if n < 0 {
		panic(n)
	}
	fmt.Println("after")
}

// Output:
// before
