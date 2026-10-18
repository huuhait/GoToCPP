// phase: 08
// ExitCode: 2
// Stderr: ^panic: send on closed channel\n\ngoroutine 1 \[running\]:\n

// A send case on a closed channel panics when it is the one the select takes (the only ready case).
package main

import "fmt"

func main() {
	closed := make(chan int, 1)
	close(closed)
	idle := make(chan int)
	fmt.Println("selecting")
	select {
	case closed <- 1:
		fmt.Println("never")
	case <-idle:
		fmt.Println("never either")
	}
}


// Output:
// selecting
