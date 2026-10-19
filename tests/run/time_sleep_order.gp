// phase: 08

// Sleep: goroutines that sleep for 10, 20 and 30 ms (started in the opposite order) report in order of their durations.
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	out := make(chan int, 3)
	// the deadlines are absolute (a common base), so the delay of starting a goroutine does not change their order
	base := time.Now().Add(400 * time.Millisecond)
	for _, ms := range []int{30, 20, 10} {
		go func() {
			time.Sleep(time.Until(base.Add(time.Duration(ms) * 10 * time.Millisecond)))
			out <- ms
		}()
	}
	for i := 0; i < 3; i++ {
		fmt.Println(<-out)
	}
	el := time.Since(start)
	fmt.Println(el >= 300*time.Millisecond, el < 10*time.Second)
	// many sleepers: all of them wake, whatever the order (the order is checked above)
	const n = 40
	res := make(chan int, n)
	for i := n - 1; i >= 0; i-- {
		go func() {
			time.Sleep(time.Duration(i+1) * time.Millisecond)
			res <- i
		}()
	}
	sum := 0
	for i := 0; i < n; i++ {
		sum += <-res
	}
	fmt.Println(sum)
	// Sleep(0) and negative durations return at once; the goroutine is not blocked
	s0 := time.Now()
	time.Sleep(0)
	time.Sleep(-time.Hour)
	time.Sleep(-1 << 63)
	fmt.Println(time.Since(s0) < 5*time.Second)
	// Sleep waits at least its duration, measured with the monotonic clock
	for _, d := range []time.Duration{time.Microsecond, 100 * time.Microsecond, time.Millisecond, 5 * time.Millisecond} {
		t := time.Now()
		time.Sleep(d)
		fmt.Println(d, time.Since(t) >= d)
	}
}

// Output:
// 10
// 20
// 30
// true true
// 780
// true
// 1µs true
// 100µs true
// 1ms true
// 5ms true
