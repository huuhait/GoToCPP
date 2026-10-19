// phase: 08

// time.After in a select: the timer wins against a channel that never delivers, and loses against one that is ready.
package main

import (
	"fmt"
	"time"
)

func main() {
	never := make(chan int)
	ready := make(chan int, 1)
	ready <- 7
	select {
	case v := <-never:
		fmt.Println("never", v)
	case <-time.After(10 * time.Millisecond):
		fmt.Println("timeout")
	}
	select {
	case v := <-ready:
		fmt.Println("ready", v)
	case <-time.After(time.Hour):
		fmt.Println("timeout")
	}
	// default with a timer that has not expired, and one that has
	t := time.NewTimer(time.Hour)
	select {
	case <-t.C:
		fmt.Println("fired")
	default:
		fmt.Println("not yet")
	}
	t.Reset(time.Millisecond)
	fired := false
	for i := 0; i < 100000 && !fired; i++ { // polls (no upper bound on the delay of the timer thread)
		select {
		case <-t.C:
			fired = true
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if fired {
		fmt.Println("fired")
	} else {
		fmt.Println("not yet")
	}
	// the loop with a ticker and a deadline
	tk := time.NewTicker(10 * time.Millisecond)
	defer tk.Stop()
	deadline := time.After(time.Hour)
	ticks := 0
loop:
	for {
		select {
		case <-tk.C:
			ticks++
			if ticks == 5 {
				break loop
			}
		case <-deadline:
			break loop
		}
	}
	fmt.Println(ticks)
	// a send to a full channel with a timeout
	full := make(chan int)
	select {
	case full <- 1:
		fmt.Println("sent")
	case <-time.After(5 * time.Millisecond):
		fmt.Println("send timed out")
	}
}

// Output:
// timeout
// ready 7
// not yet
// fired
// 5
// send timed out
