// phase: 08

// A select that waits for a channel and a timer, in a loop: asleep with a timer that can fire.
package main

import (
	"fmt"
	"time"
)

func main() {
	never := make(chan int)
	tick := time.NewTicker(3 * time.Millisecond)
	defer tick.Stop()
	timeout := time.After(40 * time.Millisecond)
	n := 0
loop:
	for {
		select {
		case <-never:
		case <-tick.C:
			n++
		case <-timeout:
			break loop
		}
	}
	fmt.Println("ticks seen:", n > 2)
}

// Output:
// ticks seen: true
