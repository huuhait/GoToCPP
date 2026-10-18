// phase: 08
// ExitCode: 2
// Stderr: ^panic: close of closed channel\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

func main() {
	c := make(chan int, 1)
	close(c)
	fmt.Println("closed once")
	close(c)
}

// Output:
// closed once
