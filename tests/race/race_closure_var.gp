// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_closure_var\.g[op]:(20|25)\b

// A goroutine and main write a variable that the goroutine's closure captured.
package main

import "fmt"

func main() {
	x := 0
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			x = 1
		}
		done <- true
	}()
	for k := 0; k < 100; k++ {
		x = 2
	}
	<-done
	fmt.Println("done", x > 0)
}

// Output:
// done true
