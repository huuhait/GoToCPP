// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_map_write\.g[op]:(23|30)\b

// Two goroutines write to the same map, one after the other with nothing between them (a concurrent write would be the runtime's "concurrent map writes" fatal error, as in Go).
package main

import (
	"fmt"
	"time"
)

var m = map[int]int{}

func main() {
	go func() {
		for k := 0; k < 100; k++ {
			m[k] = k
		}
	}()
	time.Sleep(100 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			m[k] = -k
		}
		done <- true
	}()
	<-done
	fmt.Println("done")
}

// Output:
// done
