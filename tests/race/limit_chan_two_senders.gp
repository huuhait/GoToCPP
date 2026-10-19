// phase: 08

// KNOWN LIMIT (docs/guides/race.md): Go reports this race (two senders to one channel are not ordered, and x is
// written by one before its send and by the other after its send); gocpp does not: a channel is one
// mutex, and every operation on it is ordered with the one before it. The test pins the limit; when
// channels get the edges of Go's memory model one by one, it becomes a race_ test.
package main

import (
	"fmt"
	"time"
)

var x int

func main() {
	ch := make(chan int, 10)
	done := make(chan bool)
	go func() {
		x = 1
		ch <- 1
		done <- true
	}()
	time.Sleep(50 * time.Millisecond)
	go func() {
		ch <- 2
		x = 2
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done", x > 0)
}

// Output:
// done true
