// phase: 08
// ExitCode: 2
// Stderr: ^panic: close of closed channel\n\ngoroutine 1 \[running\]:\n

// A loop that closes the channel it ranges over: the first close is fine and the loop drains the
// buffer, the second close panics.
package main

import "fmt"

func main() {
	c := make(chan int, 3)
	c <- 1
	c <- 2
	c <- 3
	for v := range c {
		fmt.Println("v", v)
		close(c)
	}
}

// Output:
// v 1
// v 2
