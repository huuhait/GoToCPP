// phase: 08

// 2000 goroutines contend for one mutex, one RWMutex and one WaitGroup at once.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	var rw sync.RWMutex
	var wg sync.WaitGroup
	counter, shared := 0, 0
	for g := 0; g < 2000; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				mu.Lock()
				counter++
				mu.Unlock()
				if i%10 == 0 {
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
	fmt.Println(counter, shared)
}

// Output:
// 100000 10000
