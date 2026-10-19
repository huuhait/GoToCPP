// phase: 08

// Go 1.23 timer channels: no stale value after Stop or Reset, even when the timer had already fired.
package main

import (
	"fmt"
	"time"
)

func main() {
	// Reset after the expiry that nobody received: the old value is gone, the new one comes after the new deadline
	t := time.NewTimer(5 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	active := t.Reset(60 * time.Millisecond)
	v := <-t.C
	fmt.Println(active, time.Since(start) >= 55*time.Millisecond, v.After(start))
	// Stop with an unreceived value, then Reset with a long delay: the receive must not return the old value
	t = time.NewTimer(5 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	fmt.Println(t.Stop())
	t.Reset(80 * time.Millisecond)
	start = time.Now()
	v = <-t.C
	fmt.Println(time.Since(start) >= 70*time.Millisecond, v.After(start))
	// a loop that always Stops and Resets without draining
	t = time.NewTimer(time.Hour)
	for i := 0; i < 100; i++ {
		t.Reset(time.Millisecond)
		time.Sleep(2 * time.Millisecond)
		if !t.Stop() {
			fmt.Println("Stop returned false at", i)
		}
	}
	fmt.Println(len(t.C), t.Stop())
	// Reset on a pending timer reports true; the timer fires once at the new time
	t = time.NewTimer(20 * time.Millisecond)
	fmt.Println(t.Reset(40 * time.Millisecond))
	start = time.Now()
	<-t.C
	fmt.Println(time.Since(start) >= 35*time.Millisecond)
	// no second value follows
	time.Sleep(60 * time.Millisecond)
	fmt.Println(len(t.C), t.Stop())
	// the time value is not older than the Reset
	t = time.NewTimer(time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	before := time.Now()
	t.Reset(time.Millisecond)
	v = <-t.C
	fmt.Println(!v.Before(before))
}

// Output:
// true true true
// true
// true true
// 0 false
// true
// true
// 0 false
// true
