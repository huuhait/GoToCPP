// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_sleep_sequential\.g[op]:(23|30)\b

// Two goroutines write a variable one after the other, 50 ms apart: nothing orders them (a sleep is no synchronization).
package main

import (
	"fmt"
	"time"
)

var x int

func main() {
	go func() {
		for i := 0; i < 100; i++ {
			x++
		}
	}()
	time.Sleep(100 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			x++
		}
		done <- true
	}()
	<-done
	fmt.Println("done")
}

// Output:
// done
