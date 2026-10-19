// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_bool_flag\.g[op]:(21|26)\b

// A flag is set by one goroutine and read by main without synchronization.
package main

import "fmt"

func main() {
	finished := false
	sink := []bool{false}
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			finished = true
		}
		done <- true
	}()
	for k := 0; k < 100; k++ {
		sink[0] = finished
	}
	<-done
	fmt.Println("done", len(sink))
}

// Output:
// done 1
