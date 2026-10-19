// phase: 08

// 100 goroutines add to one counter under a sync.Mutex, 10000 times each: no increment is lost.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	var wg sync.WaitGroup
	counter := 0
	for g := 0; g < 100; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println("counter:", counter)
}

// Output:
// counter: 1000000
