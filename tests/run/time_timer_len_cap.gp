// phase: 08

// Go 1.23 timer channels are synchronous: len and cap of C are 0 for Timer, Ticker, After and Tick,
// whether the timer is pending, has fired, or was stopped.
package main

import (
	"fmt"
	"time"
)

func lc(c <-chan time.Time) string { return fmt.Sprint(len(c), cap(c)) }

func main() {
	t := time.NewTimer(time.Hour)
	fmt.Println("pending timer", lc(t.C))
	t.Stop()
	fmt.Println("stopped timer", lc(t.C))

	f := time.NewTimer(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println("fired, unread", lc(f.C))
	<-f.C
	fmt.Println("fired, read", lc(f.C))

	// Stop drops a value that nobody received
	s := time.NewTimer(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	s.Stop()
	select {
	case <-s.C:
		fmt.Println("stale value after Stop")
	default:
		fmt.Println("no value after Stop", lc(s.C))
	}

	tk := time.NewTicker(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println("ticker, many ticks missed", lc(tk.C))
	tk.Stop()
	fmt.Println("ticker stopped", lc(tk.C))

	a := time.After(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println("After fired", lc(a))
	<-a

	tc := time.Tick(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println("Tick", lc(tc))

	af := time.AfterFunc(time.Hour, func() {})
	fmt.Println("AfterFunc has no channel", af.C == nil)
	af.Stop()

	var c <-chan time.Time = f.C
	fmt.Println(c != nil, lc(c))
}

// Output:
// pending timer 0 0
// stopped timer 0 0
// fired, unread 0 0
// fired, read 0 0
// no value after Stop 0 0
// ticker, many ticks missed 0 0
// ticker stopped 0 0
// After fired 0 0
// Tick 0 0
// AfterFunc has no channel true
// true 0 0
