// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_go_hidden_edge\.g[op]:(24|33)\b

// A goroutine writes a variable and starts another goroutine; main starts one too and reads the variable: starting goroutines orders nothing between the two.
package main

import (
	"fmt"
	"time"
)

var x int

func main() {
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			x = 1
		}
		go func() {}()
		done <- true
	}()
	time.Sleep(50 * time.Millisecond)
	go func() {}()
	y := 0
	for k := 0; k < 100; k++ {
		y += x
	}
	<-done
	fmt.Println("done", y >= 0)
}

// Output:
// done true
