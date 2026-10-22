// phase: 08

// Range runs in many goroutines while others store new keys and delete some: a key that was in the
// map for the whole run is shown by every Range, no key is shown twice in one Range, a Range that
// stops early stops at once, and a key that was deleted before the Range began is not shown.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var m sync.Map
	const stable = 200
	for i := 0; i < stable; i++ {
		m.Store(i, i)
	}
	for i := 1000; i < 1100; i++ {
		m.Store(i, i)
		m.Delete(i)
	}

	var wg sync.WaitGroup
	var missed, twice, deletedSeen, overrun atomic.Int32
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < 40; r++ {
				seen := map[int]int{}
				m.Range(func(k, v any) bool {
					key := k.(int)
					seen[key]++
					if key >= 1000 && key < 1100 {
						deletedSeen.Add(1)
					}
					return true
				})
				for i := 0; i < stable; i++ {
					if seen[i] == 0 {
						missed.Add(1)
					}
				}
				for _, n := range seen {
					if n > 1 {
						twice.Add(1)
					}
				}
				calls := 0
				m.Range(func(k, v any) bool {
					calls++
					return calls < 3
				})
				if calls > 3 {
					overrun.Add(1)
				}
			}
		}()
	}
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 400; i++ {
				m.Store(10000+g*1000+i, i)
				if i%3 == 0 {
					m.Delete(10000 + g*1000 + i)
				}
			}
		}()
	}
	wg.Wait()

	count := 0
	m.Range(func(k, v any) bool {
		count++
		return true
	})
	fmt.Println("missed:", missed.Load(), "twice:", twice.Load(), "deleted seen:", deletedSeen.Load(),
		"overrun:", overrun.Load())
	fmt.Println("count:", count)
}

// Output:
// missed: 0 twice: 0 deleted seen: 0 overrun: 0
// count: 1264
