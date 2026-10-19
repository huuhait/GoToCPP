// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// Go 1.23: a Ticker that nobody waits on does not keep the program alive (its channel is not
// referenced by a waiting goroutine), so a program that only waits on another channel is a deadlock.
package main

import (
	"fmt"
	"time"
)

func main() {
	t := time.NewTicker(time.Millisecond)
	_ = t
	fmt.Println("ticker running")
	<-make(chan int)
}

// Output:
// ticker running
