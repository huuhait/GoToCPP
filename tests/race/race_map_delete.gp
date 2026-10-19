// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_map_delete\.g[op]:(22|29)\b

// One goroutine deletes from a map, and another one inserts later: nothing orders them.
package main

import (
	"fmt"
	"time"
)

func main() {
	m := map[int]int{1: 1, 2: 2}
	go func() {
		for k := 0; k < 100; k++ {
			delete(m, 1)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			m[1] = k
		}
		done <- true
	}()
	<-done
	fmt.Println("done")
}

// Output:
// done
