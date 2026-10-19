// phase: 08
// ExitCode: 66
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_at_exit\.g[op]:(25)\b

// Goroutines race when main returns: the report is written before the program ends, and so is the count.
package main

import (
	"fmt"
	"time"
)

var x int

func main() {
	ready := make(chan bool, 2)
	for g := 0; g < 2; g++ {
		go func() {
			ready <- true
			for {
				x++
			}
		}()
	}
	<-ready
	<-ready
	time.Sleep(20 * time.Millisecond)
	fmt.Println("main returns")
}

// Output:
// main returns
