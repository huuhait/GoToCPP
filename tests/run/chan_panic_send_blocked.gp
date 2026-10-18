// phase: 08
// ExitCode: 2
// Stderr: ^panic: send on closed channel\n\ngoroutine \d+ \[running\]:\n

// A sender that is blocked when the channel is closed panics too, in its own goroutine.
package main

import "fmt"

func main() {
	c := make(chan int)
	started := make(chan bool)
	go func() {
		started <- true
		c <- 1
	}()
	<-started
	// the goroutine is about to block in the send (or already is); closing wakes it up, or makes its
	// send panic at once: either way it panics
	close(c)
	block := make(chan int)
	fmt.Println("main waits")
	<-block
}

// Output:
// main waits
