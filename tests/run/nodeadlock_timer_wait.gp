// phase: 08

// Main waits only on a timer: not a deadlock, the timer fires (Go 1.23's rule: a channel that a
// goroutine waits on keeps its timer alive).
package main

import (
	"fmt"
	"time"
)

func main() {
	<-time.After(20 * time.Millisecond)
	fmt.Println("after")
	t := time.NewTimer(20 * time.Millisecond)
	<-t.C
	fmt.Println("timer")
	t.Reset(10 * time.Millisecond)
	<-t.C
	fmt.Println("reset")
}

// Output:
// after
// timer
// reset
