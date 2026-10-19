// phase: 08

// Many goroutines store, load and delete in one sync.Map; the final contents are exact.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var m sync.Map
	var wg sync.WaitGroup
	// Disjoint keys.
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				m.Store(g*1000+i, g*1000+i)
			}
		}()
	}
	wg.Wait()
	// Racing LoadOrStore on the same keys: exactly one store wins for each key.
	var stored atomic.Int32
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 100; k++ {
				if _, loaded := m.LoadOrStore(-k-1, g); !loaded {
					stored.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	// Delete the even keys of the first group while others read.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if (i+g)%4 == 0 {
					m.Delete(i)
				}
				m.Load(i)
			}
		}()
	}
	wg.Wait()
	count, sum := 0, 0
	m.Range(func(k, v any) bool {
		count++
		if k.(int) >= 1000 {
			sum += v.(int)
		}
		return true
	})
	fmt.Println("stored:", stored.Load(), "count:", count, "sum of the big keys:", sum)
}

// Output:
// stored: 100 count: 3900 sum of the big keys: 38378100
