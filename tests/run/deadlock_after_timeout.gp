// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A select with a timeout is not asleep for ever; the receive that follows it, with the timer
// gone, is.
package main

import (
	"fmt"
	"time"
)

func main() {
	c := make(chan int)
	select {
	case <-c:
	case <-time.After(20 * time.Millisecond):
		fmt.Println("timeout")
	}
	<-c
}

// Output:
// timeout
