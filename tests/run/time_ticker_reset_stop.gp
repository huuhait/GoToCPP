// phase: 08

// Ticker.Reset and Stop: a reset changes the period, a stopped ticker is silent, a reset restarts
// it, and a ticker with a non-positive period panics.
package main

import (
	"fmt"
	"time"
)

func main() {
	tk := time.NewTicker(time.Hour)
	tk.Reset(2 * time.Millisecond)
	for i := 0; i < 3; i++ {
		<-tk.C
	}
	fmt.Println("three ticks after Reset")
	tk.Stop()
	time.Sleep(20 * time.Millisecond)
	select {
	case <-tk.C:
		fmt.Println("tick after Stop")
	default:
		fmt.Println("silent after Stop")
	}
	tk.Reset(time.Millisecond)
	<-tk.C
	fmt.Println("tick after restart")
	tk.Stop()
	tk.Stop() // stopping twice is fine

	for _, d := range []time.Duration{0, -1} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			tk.Reset(d)
		}()
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			time.NewTicker(d)
		}()
	}
	fmt.Println(len(tk.C), cap(tk.C))
}

// Output:
// three ticks after Reset
// silent after Stop
// tick after restart
// recovered: non-positive interval for Ticker.Reset
// recovered: non-positive interval for NewTicker
// recovered: non-positive interval for Ticker.Reset
// recovered: non-positive interval for NewTicker
// 0 0
