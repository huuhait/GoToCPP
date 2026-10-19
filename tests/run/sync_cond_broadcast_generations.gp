// phase: 08
// lsan: cycle

// A Cond used as a generation barrier: every round all workers wait for the last arrival, then continue together.
package main

import (
	"fmt"
	"sync"
)

type barrier struct {
	mu    sync.Mutex
	cond  *sync.Cond
	n     int
	count int
	gen   int
}

func newBarrier(n int) *barrier {
	b := &barrier{n: n}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *barrier) wait() {
	b.mu.Lock()
	defer b.mu.Unlock()
	gen := b.gen
	b.count++
	if b.count == b.n {
		b.gen++
		b.count = 0
		b.cond.Broadcast()
		return
	}
	for gen == b.gen {
		b.cond.Wait()
	}
}

func main() {
	const workers, rounds = 6, 50
	b := newBarrier(workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	arrived := make([]int, rounds)
	bad := 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				mu.Lock()
				arrived[r]++
				mu.Unlock()
				b.wait()
				mu.Lock()
				if arrived[r] != workers {
					bad++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println("bad:", bad, "generations:", b.gen)
}

// Output:
// bad: 0 generations: 50
