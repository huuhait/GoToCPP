// phase: 08

// Atomics order memory: data written before a Store is visible after the Load that sees it (the
// flag pattern), and Load/Store of a flag between two goroutines works without any lock.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	for round := 0; round < 200; round++ {
		var ready atomic.Int32
		data := make([]int, 100)
		var wg sync.WaitGroup
		wg.Add(1)
		bad := false
		go func() {
			defer wg.Done()
			for ready.Load() == 0 {
			}
			for i, v := range data {
				if v != i+round {
					bad = true
				}
			}
		}()
		for i := range data {
			data[i] = i + round
		}
		ready.Store(1)
		wg.Wait()
		if bad {
			fmt.Println("round", round, "saw stale data")
			return
		}
	}
	fmt.Println("200 rounds: the data was always visible")

	// Ping-pong with a turn variable.
	var turn atomic.Int32
	var log [20]int
	var wg sync.WaitGroup
	for p := 0; p < 2; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				for turn.Load() != int32(p) {
				}
				log[i*2+p] = p + 1
				turn.Store(int32(1 - p))
			}
		}()
	}
	wg.Wait()
	fmt.Println(log)
}

// Output:
// 200 rounds: the data was always visible
// [1 2 1 2 1 2 1 2 1 2 1 2 1 2 1 2 1 2 1 2]
