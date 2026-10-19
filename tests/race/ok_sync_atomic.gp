// phase: 08

// sync/atomic counters, typed and function style.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var a atomic.Int64
	var b int64
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				a.Add(1)
				atomic.AddInt64(&b, 2)
			}
		}()
	}
	wg.Wait()
	fmt.Println(a.Load(), atomic.LoadInt64(&b))
}

// Output:
// 800 1600
