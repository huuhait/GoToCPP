// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_buffered_early_read\.g[op]:(23|29)\b

// A value in a buffered channel is read after the receive, but written after the send.
package main

import "fmt"

var data int

func main() {
	ch := make(chan bool, 1)
	done := make(chan bool)
	go func() {
		<-ch
		for k := 0; k < 100; k++ {
			data++
		}
		done <- true
	}()
	ch <- true
	for k := 0; k < 100; k++ {
		data++
	}
	<-done
	fmt.Println("done")
}

// Output:
// done
