// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_struct_field\.g[op]:(26)\b

// Two goroutines write the same field of a struct.
package main

import "fmt"

type Stats struct {
	hits   int
	misses int
}

func main() {
	s := &Stats{}
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				s.hits++
			}
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done", s.misses)
}

// Output:
// done 0
