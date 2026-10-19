// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_slice_append\.g[op]:(22)\b

// Two goroutines append to the same slice variable.
package main

import "fmt"

var items []int

func main() {
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				items = append(items, j)
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
