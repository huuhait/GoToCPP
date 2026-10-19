// phase: 08

// Signal wakes one waiter, the one that has waited longest: the wake-up order is the order of Wait.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	waiting := 0
	var woken []int
	var wg sync.WaitGroup
	const n = 6
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			waiting++
			cond.Wait()
			woken = append(woken, i)
			mu.Unlock()
		}()
		// Start the next one only when this one is waiting: taking the lock again proves
		// that its Wait has released it.
		for {
			mu.Lock()
			w := waiting
			mu.Unlock()
			if w == i+1 {
				break
			}
		}
	}
	for i := 0; i < n; i++ {
		mu.Lock()
		before := len(woken)
		mu.Unlock()
		cond.Signal()
		for {
			mu.Lock()
			now := len(woken)
			mu.Unlock()
			if now == before+1 {
				break
			}
		}
	}
	wg.Wait()
	fmt.Println(woken)
}

// Output:
// [0 1 2 3 4 5]
