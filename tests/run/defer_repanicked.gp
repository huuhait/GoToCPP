// phase: 03
// ExitCode: 2
// Stderr: ^panic: first\n\tpanic: level \[recovered, repanicked\]\n\tpanic: -1\n\ngoroutine 1 \[running\]:\n

// Go prints two neighbouring panics of a chain whose values have the same interface words once,
// with " [recovered, repanicked]", also without recover: here the constant "level" at each of
// 200 levels (one line), and n, a local variable that only its declaration gives the constant
// -1, which gc treats as that constant. "first" and the last -1 differ from their neighbours.
package main

import "fmt"

func level(n int) {
	defer panic("level")
	if n > 0 {
		level(n - 1)
		return
	}
	panic("level")
}

func main() {
	n := -1
	defer panic(n)
	defer level(199)
	defer fmt.Println("main's defers run")
	panic("first")
}

// Output:
// main's defers run
