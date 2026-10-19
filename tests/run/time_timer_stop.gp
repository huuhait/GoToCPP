// phase: 08

// Timer.Stop: true when it stops a pending timer, false after the expiry was received or the timer was stopped; no stale value afterwards.
package main

import (
	"fmt"
	"time"
)

func main() {
	t := time.NewTimer(time.Hour)
	fmt.Println(t.Stop(), t.Stop(), t.Stop())
	// stopped before it fires: nothing is ever delivered, and a receive blocks
	t = time.NewTimer(20 * time.Millisecond)
	fmt.Println(t.Stop())
	got := make(chan bool, 1)
	go func() {
		<-t.C
		got <- true
	}()
	time.Sleep(60 * time.Millisecond)
	fmt.Println(len(got))
	// ... until it is reset
	fmt.Println(t.Reset(5 * time.Millisecond))
	fmt.Println(<-got)
	// Go 1.23: a timer that fired but whose value was not received is still "active" for Stop, and the value is dropped
	t = time.NewTimer(10 * time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	fmt.Println(len(t.C), t.Stop())
	go func() {
		<-t.C
		got <- false
	}()
	time.Sleep(60 * time.Millisecond)
	fmt.Println(len(got))
	t.Reset(time.Millisecond)
	fmt.Println(<-got)
	// Stop after the value was received
	t = time.NewTimer(5 * time.Millisecond)
	<-t.C
	fmt.Println(t.Stop())
	// the zero Timer panics, and so does a Timer that was never created
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		var z time.Timer
		z.Stop()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		var z time.Timer
		z.Reset(time.Second)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		z := &time.Timer{}
		z.Stop()
	}()
	// Stop does not close the channel
	t = time.NewTimer(time.Hour)
	t.Stop()
	fmt.Println(len(t.C), cap(t.C))
}

// Output:
// true false false
// true
// 0
// false
// true
// 0 true
// 0
// false
// false
// recovered: time: Stop called on uninitialized Timer
// recovered: time: Reset called on uninitialized Timer
// recovered: time: Stop called on uninitialized Timer
// 0 0
