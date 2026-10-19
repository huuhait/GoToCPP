// phase: 09
// ExitCode: 3
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?s)==================\n\z
// Stderr: race_exit_after_race\.g[op]:(27)\b

// os.Exit(3) after a race ends with 3, as with Go's detector (only a process that ends with 0 ends with 66
// and prints the count: the report is the last thing on stderr); the report is written first.
package main

import (
	"fmt"
	"os"
	"sync"
)

var x int

func main() {
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				x++
			}
		}()
	}
	wg.Wait()
	fmt.Println("exiting")
	os.Exit(3)
}

// Output:
// exiting
