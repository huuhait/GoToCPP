// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_main_goroutine_race\.g[op]:(21|26)\b

// Main and a goroutine race, and the goroutine is only waited for later.
package main

import "fmt"

var ready int

func main() {
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			ready = 1
		}
		done <- true
	}()
	for i := 0; i < 10; i++ {
		ready = 2
	}
	<-done
	fmt.Println("done")
}

// Output:
// done
