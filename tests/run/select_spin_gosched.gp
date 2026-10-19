// phase: 08

// A non-blocking select polled with Gosched sees the value that another goroutine sends later, and
// a send-case with default reports a full buffer.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	c := make(chan int)
	go func() { c <- 99 }()
	polls := 0
	for {
		select {
		case v := <-c:
			fmt.Println("got", v, polls >= 0)
			goto sendPhase
		default:
			polls++
			runtime.Gosched()
		}
	}
sendPhase:
	buf := make(chan int, 3)
	sent := 0
	for i := 0; i < 10; i++ {
		select {
		case buf <- i:
			sent++
		default:
		}
	}
	fmt.Println("sent", sent, "len", len(buf), "cap", cap(buf))
}

// Output:
// got 99 true
// sent 3 len 3 cap 3
