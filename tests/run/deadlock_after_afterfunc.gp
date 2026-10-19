// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// An AfterFunc that is armed keeps the program alive; once it has run (and its goroutine has
// ended) and nothing else can wake main, the program is a deadlock.
package main

import (
	"fmt"
	"time"
)

func main() {
	time.AfterFunc(20*time.Millisecond, func() { fmt.Println("the timer function ran") })
	<-make(chan int)
}

// Output:
// the timer function ran
