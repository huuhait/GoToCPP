// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_wg_sequential\.g[op]:(27|35)\b

// WaitGroup.Add after a Done is no synchronization with what the first goroutine did: only Done before Wait is.
package main

import (
	"fmt"
	"sync"
	"time"
)

var x int

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			x++
		}
	}()
	time.Sleep(50 * time.Millisecond)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			x++
		}
	}()
	wg.Wait()
	fmt.Println("done")
}

// Output:
// done
