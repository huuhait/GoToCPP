// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function recovered a loop body panic and did not resume panicking\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

func seq(yield func(int) bool) {
	defer func() { fmt.Println("iterator swallowed", recover()) }()
	yield(1)
}

func main() {
	for x := range seq {
		fmt.Println("body", x)
		panic("body panic")
	}
	fmt.Println("not reached")
}

// Output:
// body 1
// iterator swallowed body panic
