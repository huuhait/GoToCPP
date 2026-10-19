// phase: 08

// Many goroutines hand a Mutex and a WaitGroup over and over: at every moment someone is about
// to wake someone, and the detector must never see "all asleep".
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	var wg sync.WaitGroup
	total := 0
	for g := 0; g < 24; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 4000; i++ {
				mu.Lock()
				total++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println(total)

	var rw sync.RWMutex
	shared := 0
	for g := 0; g < 12; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				if i%4 == 0 {
					rw.Lock()
					shared++
					rw.Unlock()
				} else {
					rw.RLock()
					_ = shared
					rw.RUnlock()
				}
			}
		}()
	}
	wg.Wait()
	fmt.Println(shared)
}

// Output:
// 96000
// 6000
