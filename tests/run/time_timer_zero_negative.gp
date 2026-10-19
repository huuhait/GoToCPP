// phase: 08

// Timers and sleeps with a zero or negative duration are due at once.
package main

import (
	"fmt"
	"time"
)

func main() {
	for _, d := range []time.Duration{0, -1, -time.Hour} {
		t := time.NewTimer(d)
		<-t.C
		<-time.After(d)
		start := time.Now()
		time.Sleep(d)
		fmt.Println(d, time.Since(start) < 500*time.Millisecond)
		done := make(chan bool)
		time.AfterFunc(d, func() { done <- true })
		fmt.Println(<-done)
		t.Reset(d)
		<-t.C
	}
	fmt.Println("ok")
}

// Output:
// 0s true
// true
// -1ns true
// true
// -1h0m0s true
// true
// ok
