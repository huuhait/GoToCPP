// phase: 08

// NewTimer: C, len and cap (0, as in Go 1.23), the value received, Stop and Reset after the timer fired.
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	t := time.NewTimer(20 * time.Millisecond)
	fmt.Println(len(t.C), cap(t.C), t.C != nil)
	v := <-t.C
	fmt.Println(v.After(start), time.Since(start) >= 20*time.Millisecond)
	// after the value was received: Stop reports false, Reset reports false
	fmt.Println(t.Stop(), t.Stop())
	fmt.Println(t.Reset(10 * time.Millisecond))
	<-t.C
	fmt.Println(t.Reset(10*time.Millisecond), t.Stop(), t.Stop())
	// the channel stays empty after Stop
	time.Sleep(30 * time.Millisecond)
	fmt.Println(len(t.C), cap(t.C))
	// a timer with d <= 0 fires at once
	for _, d := range []time.Duration{0, -1, -time.Hour} {
		z := time.NewTimer(d)
		<-z.C
		fmt.Println(d, "fired", z.Stop())
	}
	// the channel is receive-only
	var r <-chan time.Time = t.C
	fmt.Printf("%T %v\n", r, r == t.C)
	// several timers with their own channels
	a, b := time.NewTimer(30*time.Millisecond), time.NewTimer(10*time.Millisecond)
	fmt.Println(a.C != b.C)
	tb := <-b.C
	ta := <-a.C
	fmt.Println(tb.Before(ta))
	// the time sent is the time of the expiry
	t2 := time.NewTimer(10 * time.Millisecond)
	s := time.Now()
	time.Sleep(50 * time.Millisecond)
	e := <-t2.C
	fmt.Println(e.Sub(s) < 40*time.Millisecond, e.After(s), time.Since(e) >= 30*time.Millisecond)
}

// Output:
// 0 0 true
// true true
// false false
// false
// false true false
// 0 0
// 0s fired false
// -1ns fired false
// -1h0m0s fired false
// <-chan time.Time true
// true
// true
// true true true
