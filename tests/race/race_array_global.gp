// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_array_global\.g[op]:(21)\b

// Two goroutines write the same element of a global array.
package main

import "fmt"

var table [8]int

func main() {
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			table[3] = i
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done", len(table))
}

// Output:
// done 8
