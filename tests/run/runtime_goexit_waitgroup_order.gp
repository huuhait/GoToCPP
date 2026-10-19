// phase: 08

// Goexit in several goroutines: each one's defers run in order, wg.Done runs from a defer, and
// main sees all of them finished.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	results := make([]string, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { results[i] += "c" }()
			defer func() { results[i] += "b" }()
			results[i] = "a"
			if i%2 == 0 {
				runtime.Goexit()
			}
			results[i] += "-end"
		}()
	}
	wg.Wait()
	fmt.Println(results)
	// wg.Done runs from a defer, so main can wake before the goroutines have finished exiting
	for i := 0; runtime.NumGoroutine() > 1 && i < 20000; i++ {
		time.Sleep(time.Millisecond)
	}
	fmt.Println(runtime.NumGoroutine())
}

// Output:
// [abc a-endbc abc a-endbc abc]
// 1
