// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_method_counter\.g[op]:(19)\b

// A method with a pointer receiver called from two goroutines.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() {
	for k := 0; k < 100; k++ {
		c.n++
	}
}

func main() {
	c := &Counter{}
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				c.Inc()
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
