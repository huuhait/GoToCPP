// phase: 08

// Timers and goroutines together, joined by channels: workers that wait on timers, a ticker feeding workers, a timer reset from another goroutine.
package main

import (
	"fmt"
	"time"
)

func main() {
	// 8 workers each sleep a different time and report; the main goroutine collects in order of finish
	const n = 8
	done := make(chan int)
	for i := 0; i < n; i++ {
		go func() {
			time.Sleep(time.Duration(n-i) * 100 * time.Millisecond)
			done <- i
		}()
	}
	for i := 0; i < n; i++ {
		fmt.Print(<-done, ";")
	}
	fmt.Println()
	// a ticker consumed by a goroutine that counts, stopped by a channel
	tk := time.NewTicker(5 * time.Millisecond)
	counts := make(chan int)
	stop := make(chan bool)
	go func() {
		c := 0
		for {
			<-tk.C
			c++
			if c == 6 {
				counts <- c
				<-stop
				return
			}
		}
	}()
	fmt.Println(<-counts)
	tk.Stop()
	close(stop)
	// a timer reset by a different goroutine than the one that waits
	t := time.NewTimer(time.Hour)
	fired := make(chan time.Time)
	go func() { fired <- <-t.C }()
	time.Sleep(20 * time.Millisecond)
	s := time.Now()
	go func() { t.Reset(10 * time.Millisecond) }()
	v := <-fired
	fmt.Println(v.After(s.Add(-time.Second)), time.Since(s) >= 10*time.Millisecond)
	// a pipeline with delays: stage 1 produces every 5 ms, stage 2 consumes
	src := make(chan int)
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(5 * time.Millisecond)
			src <- i
		}
		close(src)
	}()
	sum := 0
	for {
		v, ok := <-src
		if !ok {
			break
		}
		sum += v
	}
	fmt.Println(sum)
	// many goroutines blocked on timers at once
	const m = 500
	res := make(chan int, m)
	for i := 0; i < m; i++ {
		go func() {
			tt := time.NewTimer(time.Duration(i%10+1) * 5 * time.Millisecond)
			<-tt.C
			res <- 1
		}()
	}
	total := 0
	for i := 0; i < m; i++ {
		total += <-res
	}
	fmt.Println(total)
}

// Output:
// 7;6;5;4;3;2;1;0;
// 6
// true true
// 10
// 500
