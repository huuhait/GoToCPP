// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_pointer_swap\.g[op]:(23|29)\b

// A pointer variable is replaced by one goroutine while another reads it.
package main

import "fmt"

type Config struct{ level int }

var current = &Config{level: 1}

func main() {
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			current = &Config{level: 2}
		}
		done <- true
	}()
	var c *Config
	for k := 0; k < 100; k++ {
		c = current
	}
	<-done
	fmt.Println("done", c != nil)
}

// Output:
// done true
