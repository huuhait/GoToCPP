// phase: 08

// KNOWN LIMIT (docs/guides/race.md): Go reports this race (two receivers from one channel are not ordered); gocpp does not: a
// channel is one mutex, and every operation on it is ordered with the one before it (see
// limit_chan_two_senders).
package main

import (
	"fmt"
	"time"
)

var x int

func main() {
	ch := make(chan int, 10)
	ch <- 1
	ch <- 2
	done := make(chan bool)
	go func() {
		x = 1
		<-ch
		done <- true
	}()
	time.Sleep(50 * time.Millisecond)
	go func() {
		<-ch
		x = 2
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done", x > 0)
}

// Output:
// done true
