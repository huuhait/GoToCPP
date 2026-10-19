// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_map_read_write\.g[op]:(22|28)\b

// One goroutine writes to a map, and main reads it afterwards: nothing orders them.
package main

import (
	"fmt"
	"time"
)

func main() {
	m := map[string]int{"a": 1}
	go func() {
		for k := 0; k < 100; k++ {
			m["a"] = k
		}
	}()
	time.Sleep(50 * time.Millisecond)
	v := 0
	for k := 0; k < 100; k++ {
		v += m["a"]
	}
	fmt.Println("done", v >= 0)
}

// Output:
// done true
