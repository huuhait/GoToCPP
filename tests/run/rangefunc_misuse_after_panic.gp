// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function continued iteration after loop body panic\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

func seq(yield func(int) bool) {
	func() {
		defer func() { fmt.Println("iterator swallowed", recover()) }()
		yield(1)
	}()
	yield(2)
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
