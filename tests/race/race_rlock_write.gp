// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_rlock_write\.g[op]:(29)\b

// Writing under RLock is a race between the writers.
package main

import (
	"fmt"
	"sync"
)

var mu sync.RWMutex
var x int

func main() {
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				mu.RLock()
				x++
				mu.RUnlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println("done")
}

// Output:
// done
