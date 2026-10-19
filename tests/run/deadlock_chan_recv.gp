// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A receive from a channel that nobody sends on: Go ends the program with its deadlock report
// (every goroutine, here only the main one, with the reason it waits and its stack) and exit code
// 2. What was printed before stays, and main's deferred calls do not run.
package main

import "fmt"

func main() {
	defer fmt.Println("not printed: a fatal error runs no deferred call")
	ch := make(chan int)
	fmt.Println("before")
	<-ch
	fmt.Println("not reached")
}

// Output:
// before
