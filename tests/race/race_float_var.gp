// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_float_var\.g[op]:(22)\b

// Two goroutines add to a shared float64.
package main

import "fmt"

var total float64

func main() {
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				total += 0.5
			}
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
