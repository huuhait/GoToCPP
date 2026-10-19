// phase: 08

// Broadcast wakes every waiter: N goroutines wait for a gate to open.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	open := false
	waiting := 0
	var wg sync.WaitGroup
	passed := make([]bool, 12)
	for i := range passed {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			waiting++
			for !open {
				cond.Wait()
			}
			passed[i] = true
			mu.Unlock()
		}()
	}
	// Wait until all of them wait (each one counts itself under the lock, and Wait releases it).
	for {
		mu.Lock()
		n := waiting
		mu.Unlock()
		if n == len(passed) {
			break
		}
	}
	mu.Lock()
	open = true
	mu.Unlock()
	cond.Broadcast()
	wg.Wait()
	all := true
	for _, p := range passed {
		all = all && p
	}
	fmt.Println("all passed:", all)
	// With nobody waiting, Signal and Broadcast do nothing.
	cond.Signal()
	cond.Broadcast()
	fmt.Println("done")
}

// Output:
// all passed: true
// done
