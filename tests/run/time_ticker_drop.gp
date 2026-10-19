// phase: 08

// A Ticker drops the ticks of a slow receiver: the channel holds one tick, later ticks are discarded.
package main

import (
	"fmt"
	"time"
)

func main() {
	tk := time.NewTicker(10 * time.Millisecond)
	defer tk.Stop()
	start := time.Now()
	// do not receive for a while: ticks at 10, 20, 30, 40 and 50 ms come, only the first is kept
	time.Sleep(55 * time.Millisecond)
	first := <-tk.C
	// the channel took the first tick; the next one received is the first after we started to receive again
	second := <-tk.C
	fmt.Println(first.Sub(start) < 25*time.Millisecond, second.Sub(first) >= 40*time.Millisecond)
	// a receiver that is always late gets the ticks one period apart, never a burst
	times := []time.Time{}
	for i := 0; i < 4; i++ {
		time.Sleep(25 * time.Millisecond)
		times = append(times, <-tk.C)
	}
	burst := false
	for i := 1; i < len(times); i++ {
		if times[i].Sub(times[i-1]) < 8*time.Millisecond {
			burst = true
		}
	}
	fmt.Println("burst:", burst)
	// the buffered tick does not show in len
	time.Sleep(30 * time.Millisecond)
	fmt.Println(len(tk.C), cap(tk.C))
	// Stop discards the buffered tick
	tk.Stop()
	fmt.Println(len(tk.C))
	tk.Reset(time.Hour)
	time.Sleep(10 * time.Millisecond)
	fmt.Println(len(tk.C))
}

// Output:
// true true
// burst: false
// 0 0
// 0
// 0
