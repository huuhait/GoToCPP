// phase: 08

// An AfterFunc wakes the main goroutine through a channel: main waits on the channel only.
package main

import (
	"fmt"
	"time"
)

func main() {
	done := make(chan string)
	time.AfterFunc(20*time.Millisecond, func() { done <- "from the timer function" })
	fmt.Println(<-done)
	var timers []*time.Timer
	ch := make(chan int, 3)
	for i := 1; i <= 3; i++ {
		timers = append(timers, time.AfterFunc(time.Duration(i)*5*time.Millisecond, func() { ch <- i }))
	}
	fmt.Println(<-ch + <-ch + <-ch) // (the functions may run in any order)
	_ = timers
}

// Output:
// from the timer function
// 6
