// phase: 08

// A debounce loop with Reset: events arrive far faster than the delay, so the timer fires once, after the last one.
// The sender does not sleep between events (the loop takes microseconds against a delay of a second), so the
// timer cannot fire early unless the machine stalls the loop for a whole second.
package main

import (
	"fmt"
	"time"
)

func main() {
	events := make(chan int)
	go func() {
		for i := 1; i <= 200; i++ {
			events <- i
		}
		close(events)
	}()
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	last, fired, count := 0, 0, 0
	for done := false; !done; {
		select {
		case v, ok := <-events:
			if !ok {
				done = true
				break
			}
			count++
			last = v
			timer.Reset(time.Second)
		case <-timer.C:
			fired++
			fmt.Println("fired early at", last)
		}
	}
	<-timer.C
	fired++
	fmt.Println("events", count, "last", last, "fired", fired)
	select {
	case <-timer.C:
		fmt.Println("stale value")
	default:
		fmt.Println("no stale value")
	}
}

// Output:
// events 200 last 200 fired 1
// no stale value
