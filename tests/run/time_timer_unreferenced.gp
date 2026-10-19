// phase: 08

// A Timer, Ticker or After channel that nobody holds is freed even if it was never stopped (Go 1.23): a loop that makes
// a million of them does not pile up threads, memory or heap entries, and the program ends at once.
package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Now()
	for i := 0; i < 200000; i++ {
		_ = time.NewTimer(time.Hour)
	}
	for i := 0; i < 200000; i++ {
		_ = time.After(time.Hour)
	}
	for i := 0; i < 200000; i++ {
		_ = time.NewTicker(time.Hour)
	}
	for i := 0; i < 200000; i++ {
		_ = time.Tick(time.Hour)
	}
	// a Ticker with a short period that nobody receives from is dropped too
	for i := 0; i < 1000; i++ {
		_ = time.NewTicker(time.Millisecond)
	}
	fmt.Println("made")
	// the timers that are held keep working
	t := time.NewTimer(10 * time.Millisecond)
	<-t.C
	c := time.After(5 * time.Millisecond)
	<-c
	fmt.Println("held timers fire", time.Since(start) < 5*time.Minute)
	// a channel kept without its Timer still receives
	ch := time.NewTimer(10 * time.Millisecond).C
	time.Sleep(time.Millisecond)
	<-ch
	fmt.Println("channel alone fires")
	// a func timer of which nobody holds the Timer still runs
	done := make(chan bool)
	time.AfterFunc(5*time.Millisecond, func() { done <- true })
	<-done
	fmt.Println("func timer runs")
	// the program ends with a thousand armed timers and sleeping goroutines
	for i := 0; i < 1000; i++ {
		go time.Sleep(time.Hour)
	}
	for i := 0; i < 1000; i++ {
		time.AfterFunc(time.Hour, func() {})
		_ = time.NewTimer(time.Hour)
	}
	fmt.Println("exit")
}

// Output:
// made
// held timers fire true
// channel alone fires
// func timer runs
// exit
