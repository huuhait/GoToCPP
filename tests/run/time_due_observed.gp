// phase: 08

// A timer whose deadline has passed has fired as far as the program can tell, whether the runtime's
// timer thread has run yet or not (Go 1.23): a non-blocking receive, a select with a default, Stop and
// Reset all see it. Every count below is 0 misses, whatever the load: the checks follow a Sleep or a
// busy wait that is past the deadline.
package main

import (
	"fmt"
	"time"
)

func ready(c <-chan time.Time) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

func main() {
	// a timer that is due at its creation
	fmt.Println(ready(time.After(0)), ready(time.After(-5)), ready(time.NewTimer(0).C), ready(time.NewTimer(-time.Hour).C))
	// not due yet: the default is taken
	t := time.NewTimer(time.Hour)
	fmt.Println(ready(t.C), len(t.C), cap(t.C))
	// due after a Sleep: many times
	miss := 0
	for i := 0; i < 5000; i++ {
		t := time.NewTimer(100 * time.Microsecond)
		time.Sleep(300 * time.Microsecond)
		if !ready(t.C) {
			miss++
		}
	}
	fmt.Println("sleep misses:", miss)
	// due after a busy wait
	miss = 0
	for i := 0; i < 5000; i++ {
		t := time.NewTimer(200 * time.Microsecond)
		for end := time.Now().Add(300 * time.Microsecond); time.Now().Before(end); {
		}
		if !ready(t.C) {
			miss++
		}
	}
	fmt.Println("busy misses:", miss)
	// the value is delivered once
	t = time.NewTimer(0)
	fmt.Println(ready(t.C), ready(t.C))
	// len and cap say 0 when the timer is due
	t = time.NewTimer(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	fmt.Println(len(t.C), cap(t.C))
	// Stop after the deadline: true until the value is received
	fmt.Println(t.Stop(), t.Stop(), ready(t.C))
	t = time.NewTimer(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	<-t.C
	fmt.Println(t.Stop())
	t = time.NewTimer(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	fmt.Println(ready(t.C), t.Stop())
	// Reset after the deadline: true (the value was not received), and the old value is gone
	t = time.NewTimer(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	fmt.Println(t.Reset(time.Hour), ready(t.C))
	t = time.NewTimer(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	<-t.C
	fmt.Println(t.Reset(time.Millisecond))
	time.Sleep(3 * time.Millisecond)
	fmt.Println(ready(t.C), ready(t.C))
	// a ticker whose tick is due
	miss = 0
	for i := 0; i < 300; i++ {
		tk := time.NewTicker(100 * time.Microsecond)
		time.Sleep(300 * time.Microsecond)
		if !ready(tk.C) {
			miss++
		}
		// the next tick is one period away at most, and only one tick was kept
		time.Sleep(300 * time.Microsecond)
		if !ready(tk.C) {
			miss++
		}
		tk.Stop()
		if ready(tk.C) {
			miss++
		}
	}
	fmt.Println("ticker misses:", miss)
	tk := time.NewTicker(time.Hour)
	fmt.Println(ready(tk.C), len(tk.C))
	tk.Reset(time.Microsecond)
	for end := time.Now().Add(2 * time.Millisecond); time.Now().Before(end); {
	}
	fmt.Println(ready(tk.C))
	tk.Stop()
	// a select with other cases: the due timer is chosen when nothing else is ready
	never := make(chan int)
	t = time.NewTimer(time.Millisecond)
	time.Sleep(3 * time.Millisecond)
	select {
	case <-never:
		fmt.Println("never")
	case <-t.C:
		fmt.Println("timer")
	default:
		fmt.Println("default")
	}
}

// Output:
// true true true true
// false 0 0
// sleep misses: 0
// busy misses: 0
// true false
// 0 0
// true false false
// false
// true false
// true false
// false
// true false
// ticker misses: 0
// false 0
// true
// timer
