// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_slice_elem\.g[op]:(20)\b

// Two goroutines write the same element of a slice.
package main

import "fmt"

func main() {
	a := make([]int, 4)
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			a[1] = i + 1
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done", len(a))
}

// Output:
// done 4
