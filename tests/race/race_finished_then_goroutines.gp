// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_finished_then_goroutines\.g[op]:(25|39)\b

// A goroutine writes a variable and ends; 30 other goroutines run and end; then one allocates and writes the variable: the race is found (the last 64 finished goroutines are kept).
package main

import (
	"fmt"
	"sync"
	"time"
)

var x int
var sink [][]int

func main() {
	go func() {
		for i := 0; i < 100; i++ {
			x++
		}
	}()
	time.Sleep(20 * time.Millisecond)
	for g := 0; g < 30; g++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done() }()
		wg.Wait()
	}
	for i := 0; i < 2000; i++ {
		sink = append(sink[:0], make([]int, 4))
	}
	for i := 0; i < 100; i++ {
		x++
	}
	fmt.Println("done")
}

// Output:
// done
