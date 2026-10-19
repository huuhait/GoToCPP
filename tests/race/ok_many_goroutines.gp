// phase: 08

// Three thousand goroutines in batches, each adds to a counter under a mutex: nothing is reported, and the threads of the finished goroutines do not pile up.
package main

import (
	"fmt"
	"sync"
)

var total int
var mu sync.Mutex

func main() {
	for round := 0; round < 6; round++ {
		var wg sync.WaitGroup
		for g := 0; g < 500; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mu.Lock()
				total++
				mu.Unlock()
			}()
		}
		wg.Wait()
	}
	fmt.Println(total)
}

// Output:
// 3000
