// phase: 08

// AfterFunc: the function runs in its own goroutine at the deadline; ordering; the returned Timer has no channel.
package main

import (
	"fmt"
	"time"
)

func main() {
	out := make(chan string, 10)
	start := time.Now()
	// 100 ms apart: each function runs on its own goroutine, so a delay of the machine could otherwise
	// reorder two functions that are due together
	time.AfterFunc(220*time.Millisecond, func() { out <- "c" })
	time.AfterFunc(20*time.Millisecond, func() { out <- "a" })
	t := time.AfterFunc(120*time.Millisecond, func() { out <- "b" })
	fmt.Println(t.C == nil)
	fmt.Println(<-out, <-out, <-out)
	fmt.Println(time.Since(start) >= 220*time.Millisecond)
	// a func that blocks does not block other timers
	block := make(chan bool)
	time.AfterFunc(5*time.Millisecond, func() { <-block })
	time.AfterFunc(15*time.Millisecond, func() { out <- "after the blocked one" })
	fmt.Println(<-out)
	close(block)
	// AfterFunc runs on another goroutine: it can send to an unbuffered channel the caller receives from
	unbuf := make(chan int)
	time.AfterFunc(time.Millisecond, func() { unbuf <- 42 })
	fmt.Println(<-unbuf)
	// d <= 0 runs at once
	for _, d := range []time.Duration{0, -1, -time.Hour} {
		time.AfterFunc(d, func() { out <- "now" })
		fmt.Println(<-out)
	}
	// the timer is not referenced by anyone and still fires
	time.AfterFunc(5*time.Millisecond, func() { out <- "unreferenced" })
	fmt.Println(<-out)
	// a func that panics ends the program: tested in time_afterfunc_panic
	// Reset after the function ran schedules it again, and Reset returns false then
	n := make(chan int, 10)
	count := 0
	var tm *time.Timer
	tm = time.AfterFunc(5*time.Millisecond, func() {
		count++
		n <- count
	})
	fmt.Println(<-n)
	fmt.Println(tm.Reset(5 * time.Millisecond))
	fmt.Println(<-n)
	// Reset of a pending AfterFunc timer moves it, and reports true
	tm.Reset(time.Hour)
	fmt.Println(tm.Reset(5 * time.Millisecond))
	fmt.Println(<-n, tm.Stop())
}

// Output:
// true
// a b c
// true
// after the blocked one
// 42
// now
// now
// now
// unreferenced
// 1
// false
// 2
// true
// 3 false
