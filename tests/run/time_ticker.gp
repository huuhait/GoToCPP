// phase: 08

// Ticker: ticks in order and spaced by the period, Stop, Reset, len and cap, Tick.
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	tk := time.NewTicker(10 * time.Millisecond)
	fmt.Println(len(tk.C), cap(tk.C))
	var ticks []time.Time
	for i := 0; i < 5; i++ {
		ticks = append(ticks, <-tk.C)
	}
	ordered := true
	for i := 1; i < len(ticks); i++ {
		if !ticks[i].After(ticks[i-1]) {
			ordered = false
		}
	}
	fmt.Println(ordered, ticks[0].After(start), time.Since(start) >= 50*time.Millisecond)
	// the ticks are multiples of the period after the start, not delayed by the receiver
	span := ticks[4].Sub(ticks[0])
	fmt.Println(span >= 35*time.Millisecond, span < 4*time.Second)
	tk.Stop()
	time.Sleep(40 * time.Millisecond)
	fmt.Println(len(tk.C))
	// Stop twice and a zero Ticker's Stop do not panic
	tk.Stop()
	var z time.Ticker
	z.Stop()
	// Reset changes the period, and restarts a stopped ticker
	tk.Reset(5 * time.Millisecond)
	s := time.Now()
	for i := 0; i < 4; i++ {
		<-tk.C
	}
	fmt.Println(time.Since(s) >= 15*time.Millisecond)
	tk.Reset(50 * time.Millisecond)
	s = time.Now()
	<-tk.C
	fmt.Println(time.Since(s) >= 45*time.Millisecond)
	tk.Stop()
	// Tick returns only the channel; d <= 0 gives a nil channel
	c := time.Tick(5 * time.Millisecond)
	<-c
	<-c
	fmt.Println(len(c), cap(c), c != nil)
	fmt.Println(time.Tick(0) == nil, time.Tick(-1) == nil, time.Tick(-time.Hour) == nil)
	// misuse
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		time.NewTicker(0)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		time.NewTicker(-5)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		tk.Reset(0)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		tk.Reset(-time.Second)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		var z time.Ticker
		z.Reset(time.Second)
	}()
}

// Output:
// 0 0
// true true true
// true true
// 0
// true
// true
// 0 0 true
// true true true
// recovered: non-positive interval for NewTicker
// recovered: non-positive interval for NewTicker
// recovered: non-positive interval for Ticker.Reset
// recovered: non-positive interval for Ticker.Reset
// recovered: time: Reset called on uninitialized Ticker
