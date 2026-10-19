// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_range_closure\.g[op]:(20)\b

// Goroutines started in a loop share a variable declared outside it.
package main

import "fmt"

func main() {
	var sum int
	done := make(chan bool)
	for i := 1; i <= 3; i++ {
		go func() {
			sum += i
			done <- true
		}()
	}
	for i := 0; i < 3; i++ {
		<-done
	}
	fmt.Println("done")
}

// Output:
// done
