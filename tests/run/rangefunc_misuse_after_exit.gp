// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function continued iteration after whole loop exit\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

var saved func(int) bool

func seq(yield func(int) bool) {
	saved = yield
	yield(1)
}

func main() {
	for x := range seq {
		fmt.Println("body", x)
	}
	fmt.Println("loop over")
	saved(2)
	fmt.Println("not reached")
}

// Output:
// body 1
// loop over
