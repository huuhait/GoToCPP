// bench: mutex_counter
// category: runtime
// size: 6000000
// smoke: 20000
// twin: no
// about: four goroutines increment a shared counter under a sync.Mutex
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	var wg sync.WaitGroup
	counter := 0
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < benchN/4; i++ {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println(counter)
}
