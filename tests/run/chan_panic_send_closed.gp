// phase: 08
// ExitCode: 2
// Stderr: ^panic: send on closed channel\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

func main() {
	c := make(chan int, 1)
	close(c)
	fmt.Println("sending")
	c <- 1
}

// Output:
// sending
