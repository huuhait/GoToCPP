// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_two_races\.g[op]:(21|24|29|32)\b

// Two different variables race: the program reports both.
package main

import "fmt"

var a, b int

func main() {
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			a = 1
		}
		for k := 0; k < 100; k++ {
			b = 1
		}
		done <- true
	}()
	for k := 0; k < 100; k++ {
		a = 2
	}
	for k := 0; k < 100; k++ {
		b = 2
	}
	<-done
	fmt.Println("done")
}

// Output:
// done
