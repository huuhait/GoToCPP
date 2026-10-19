// phase: 08

// Once.Do runs f exactly once, however many goroutines call it, and every call returns only after
// f has finished.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var once sync.Once
	var runs atomic.Int32
	var finishedBeforeReturn atomic.Int32
	state := 0
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			once.Do(func() {
				runs.Add(1)
				state = 42
			})
			if state == 42 {
				finishedBeforeReturn.Add(1)
			}
		}()
	}
	wg.Wait()
	fmt.Println("runs:", runs.Load(), "saw the result:", finishedBeforeReturn.Load())

	// Different functions on the same Once: only the first one runs.
	var o2 sync.Once
	o2.Do(func() { fmt.Println("first") })
	o2.Do(func() { fmt.Println("second") })
}

// Output:
// runs: 1 saw the result: 50
// first
