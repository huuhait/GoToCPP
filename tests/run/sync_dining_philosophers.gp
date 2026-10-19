// phase: 08

// Dining philosophers with the forks ordered by number: nobody deadlocks, and each philosopher
// eats exactly as often as planned. TryLock-based second attempt on the left fork too.
package main

import (
	"fmt"
	"sync"
)

func main() {
	const n = 5
	forks := make([]sync.Mutex, n)
	meals := make([]int, n)
	var wg sync.WaitGroup
	for p := 0; p < n; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, b := p, (p+1)%n
			if a > b {
				a, b = b, a
			}
			for i := 0; i < 500; i++ {
				forks[a].Lock()
				forks[b].Lock()
				meals[p]++
				forks[b].Unlock()
				forks[a].Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println(meals)

	// The same with TryLock and backoff: eat only when both forks are free.
	eaten := make([]int, n)
	for p := 0; p < n; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, r := &forks[p], &forks[(p+1)%n]
			for eaten[p] < 300 {
				if !l.TryLock() {
					continue
				}
				if r.TryLock() {
					eaten[p]++
					r.Unlock()
				}
				l.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println(eaten)
}

// Output:
// [500 500 500 500 500]
// [300 300 300 300 300]
