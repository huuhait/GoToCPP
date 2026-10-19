// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan send\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A send on an unbuffered channel that nobody receives from, and a full buffered one.
package main

import "fmt"

func main() {
	buffered := make(chan int, 2)
	buffered <- 1
	buffered <- 2
	fmt.Println("buffer full:", len(buffered), cap(buffered))
	unbuffered := make(chan string)
	unbuffered <- "nobody listens"
}

// Output:
// buffer full: 2 2
