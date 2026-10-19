// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV
// Stderr: goroutine 1 \[running\]:\n

package main

import "fmt"

func main() {
	var seq func(yield func(int) bool)
	fmt.Println("before")
	for x := range seq {
		fmt.Println(x)
	}
	fmt.Println("not reached")
}

// Output:
// before
