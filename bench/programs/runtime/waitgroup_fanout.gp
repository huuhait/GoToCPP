// bench: waitgroup_fanout
// category: runtime
// size: 400000
// smoke: 200
// twin: no
// about: fan-out of goroutines that each add to a mutex-protected map and are joined with a sync.WaitGroup
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	var wg sync.WaitGroup
	m := map[int]int{}
	for i := 0; i < benchN; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			m[i%97] += i
			mu.Unlock()
		}()
	}
	wg.Wait()
	sum := 0
	for _, v := range m {
		sum += v
	}
	fmt.Println(len(m), sum)
}
