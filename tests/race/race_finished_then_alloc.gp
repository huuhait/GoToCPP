// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_finished_then_alloc\.g[op]:(25|35)\b

// A goroutine writes a variable and ends; another one allocates a lot and then writes it: the race is still found (ThreadSanitizer drops the trace of a thread that has ended).
package main

import (
	"fmt"
	"time"
)

var x int
var sink [][]int

func main() {
	done := make(chan bool, 2)
	go func() {
		for i := 0; i < 100; i++ {
			x++
		}
		done <- true
	}()
	go func() {
		time.Sleep(20 * time.Millisecond)
		for i := 0; i < 2000; i++ {
			sink = append(sink[:0], make([]int, 4))
		}
		for i := 0; i < 100; i++ {
			x++
		}
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
