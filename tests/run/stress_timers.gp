// phase: 09

// Stress: many timers at once (GOCPP_STRESS=1: 20000; default: 2000). Half are stopped, half fire; AfterFunc
// functions run exactly once; Reset and Stop race with the expiry.
// Run it big with: GOCPP_STRESS=1 go test ./tests -run 'Run/stress_timers'
package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

func scale(small, big int) int {
	if os.Getenv("GOCPP_STRESS") == "1" {
		return big
	}
	return small
}

func main() {
	n := scale(2000, 20000)

	var ran atomic.Int64
	timers := make([]*time.Timer, n)
	for i := range timers {
		d := time.Hour
		if i%2 == 0 {
			d = time.Duration(1+i%7) * time.Millisecond
		}
		timers[i] = time.AfterFunc(d, func() { ran.Add(1) })
	}
	stopped := 0
	for i, t := range timers {
		if i%2 == 1 && t.Stop() {
			stopped++
		}
	}
	for ran.Load() < int64(n/2) {
		time.Sleep(time.Millisecond)
	}
	fmt.Println("fired:", ran.Load() == int64(n/2), "stopped:", stopped == n/2)

	// channel timers, each in its own goroutine
	var wg sync.WaitGroup
	var got atomic.Int64
	for i := 0; i < n/10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTimer(time.Hour)
			t.Reset(time.Duration(1+i%5) * time.Millisecond)
			<-t.C
			got.Add(1)
		}()
	}
	wg.Wait()
	fmt.Println("channel timers:", got.Load() == int64(n/10))

	// Stop and Reset against the expiry: exactly one of "stopped" and "fired" for every timer
	var fired atomic.Int64
	var stops atomic.Int64
	var wg2 sync.WaitGroup
	for i := 0; i < n/10; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			t := time.AfterFunc(time.Duration(i%3)*time.Millisecond, func() { fired.Add(1) })
			time.Sleep(time.Duration(i%3) * time.Millisecond)
			if t.Stop() {
				stops.Add(1)
			}
		}()
	}
	wg2.Wait()
	// the functions run on goroutines of their own: wait for the ones whose Stop returned false
	for i := 0; fired.Load()+stops.Load() < int64(n/10) && i < 20000; i++ {
		time.Sleep(time.Millisecond)
	}
	fmt.Println("stop xor fire:", fired.Load()+stops.Load() == int64(n/10))
}

// Output:
// fired: true stopped: true
// channel timers: true
// stop xor fire: true
