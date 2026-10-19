// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_defer_write\.g[op]:(20|25)\b

// A deferred write races with a write of another goroutine.
package main

import "fmt"

var state int

func worker(done chan bool) {
	defer func() {
		for k := 0; k < 100; k++ {
			state = 0
		}
		done <- true
	}()
	for k := 0; k < 100; k++ {
		state = 1
	}
}

func main() {
	done := make(chan bool)
	go worker(done)
	go worker(done)
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
