// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_slice_len\.g[op]:(21|27)\b

// One goroutine appends to a slice while another reads its length.
package main

import "fmt"

var items []int

func main() {
	done := make(chan bool)
	go func() {
		for j := 0; j < 20; j++ {
			items = append(items, j)
		}
		done <- true
	}()
	n := 0
	for k := 0; k < 100; k++ {
		n += len(items)
	}
	<-done
	_ = n
	fmt.Println("done")
}

// Output:
// done
