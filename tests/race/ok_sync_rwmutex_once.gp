// phase: 08

// RWMutex readers and a writer, and a Once.
package main

import (
	"fmt"
	"sync"
)

var (
	mu    sync.RWMutex
	table = map[int]int{}
	once  sync.Once
	setup []int
)

func main() {
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			mu.Lock()
			table[g] = g * g
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			once.Do(func() { setup = append(setup, 1) })
			mu.RLock()
			_ = table[g]
			mu.RUnlock()
		}()
	}
	wg.Wait()
	fmt.Println(len(table), len(setup))
}

// Output:
// 4 1
