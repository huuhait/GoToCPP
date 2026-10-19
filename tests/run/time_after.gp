// phase: 08

// time.After: the channel receives the time of the expiry; ordering of several After channels; After with d <= 0.
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	c := time.After(20 * time.Millisecond)
	fmt.Println(len(c), cap(c))
	v := <-c
	el := time.Since(start)
	fmt.Println(el >= 20*time.Millisecond, v.After(start), v.Before(start.Add(10*time.Second)))
	// the value is the time when the timer expired, not when it was received
	c = time.After(10 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	v = <-c
	fmt.Println(time.Since(v) >= 40*time.Millisecond, time.Since(v) < 10*time.Second)
	// three timers, received in the order of their deadlines whatever the order of the receives
	a, b, d := time.After(30*time.Millisecond), time.After(10*time.Millisecond), time.After(20*time.Millisecond)
	tb := <-b
	td := <-d
	ta := <-a
	fmt.Println(tb.Before(td), td.Before(ta), ta.Sub(tb) >= 15*time.Millisecond)
	// zero and negative durations fire at once
	for _, dd := range []time.Duration{0, -1, -time.Hour, -1 << 63} {
		t := time.Now()
		<-time.After(dd)
		fmt.Println(dd, time.Since(t) < 5*time.Second)
	}
	// After in a loop with a long timeout that is never received: nothing waits for them
	for i := 0; i < 20000; i++ {
		_ = time.After(time.Hour)
	}
	fmt.Println("done")
}

// Output:
// 0 0
// true true true
// true true
// true true true
// 0s true
// -1ns true
// -1h0m0s true
// -2562047h47m16.854775808s true
// done
