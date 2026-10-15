// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[3\] with length 3\n\ngoroutine 1 \[running\]:\n

// A panic in a loop after defers piled up in it: the deferred calls run in reverse order, each
// with the arguments of its own iteration, then the panic ends the program (exit code 2).
package main

import "fmt"

func work(xs []int) {
	for i := 0; i <= len(xs); i++ {
		defer fmt.Println("undo", i)
		fmt.Println("do", xs[i])
	}
}

func main() {
	defer fmt.Println("main's defer")
	work([]int{10, 20, 30})
	fmt.Println("not reached")
}

// Output:
// do 10
// do 20
// do 30
// undo 3
// undo 2
// undo 1
// undo 0
// main's defer
