// phase: 08

// Main waits on a Ticker's channel, then stops it; the receives are ticks, never a deadlock.
package main

import (
	"fmt"
	"time"
)

func main() {
	t := time.NewTicker(5 * time.Millisecond)
	for i := 1; i <= 4; i++ {
		<-t.C
		fmt.Println("tick", i)
	}
	t.Stop()
	for range time.Tick(time.Millisecond) {
		fmt.Println("tick from time.Tick")
		break
	}
}

// Output:
// tick 1
// tick 2
// tick 3
// tick 4
// tick from time.Tick
