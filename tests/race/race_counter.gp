// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_counter\.g[op]:(19)\b

// Two goroutines add to a shared counter without synchronization.
package main

import "fmt"

var counter int

func work(done chan bool) {
	for i := 0; i < 100; i++ {
		counter++
	}
	done <- true
}

func main() {
	done := make(chan bool)
	go work(done)
	go work(done)
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
