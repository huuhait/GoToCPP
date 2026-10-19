// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_rwmutex_reader_pair\.g[op]:(27|36)\b

// Two readers of an RWMutex write a variable: readers are not ordered with each other.
package main

import (
	"fmt"
	"sync"
	"time"
)

var x int
var mu sync.RWMutex

func main() {
	done := make(chan bool, 2)
	go func() {
		mu.RLock()
		for i := 0; i < 100; i++ {
			x = 1
		}
		mu.RUnlock()
		done <- true
	}()
	time.Sleep(50 * time.Millisecond)
	go func() {
		mu.RLock()
		for i := 0; i < 100; i++ {
			x = 2
		}
		mu.RUnlock()
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done", x > 0)
}

// Output:
// done true
