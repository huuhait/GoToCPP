// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_read_write\.g[op]:(21|27)\b

// One goroutine writes a variable that another one reads.
package main

import "fmt"

var shared int

func main() {
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			shared = 42
		}
		done <- true
	}()
	v := 0
	for k := 0; k < 100; k++ {
		v += shared
	}
	<-done
	_ = v
	fmt.Println("done")
}

// Output:
// done
