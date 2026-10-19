// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_close_no_edge\.g[op]:(22|28)\b

// A close orders nothing for a goroutine that does not wait for it.
package main

import "fmt"

var x int

func main() {
	stop := make(chan struct{})
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			x = 1
		}
		done <- true
	}()
	go func() {
		for k := 0; k < 100; k++ {
			x = 2
		}
		close(stop)
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
