// phase: 08

// Buffers taken from a Pool by several goroutines are never shared: each goroutine writes its id
// into its buffer, works, and checks that nobody changed it before it puts it back.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var created atomic.Int32
	pool := sync.Pool{New: func() any {
		created.Add(1)
		return make([]int, 64)
	}}
	var wg sync.WaitGroup
	var corrupted atomic.Int32
	var rounds atomic.Int32
	for g := 1; g <= 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				b := pool.Get().([]int)
				for k := range b {
					b[k] = g
				}
				for k := range b {
					if b[k] != g {
						corrupted.Add(1)
						break
					}
				}
				rounds.Add(1)
				pool.Put(b)
			}
		}()
	}
	wg.Wait()
	fmt.Println("rounds:", rounds.Load(), "corrupted:", corrupted.Load(), "created at most one per round:", created.Load() <= rounds.Load())
}

// Output:
// rounds: 32000 corrupted: 0 created at most one per round: true
