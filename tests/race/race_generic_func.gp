// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_generic_func\.g[op]:(17)\b

// A generic function is the racy code.
package main

import "fmt"

func Store[T any](p *T, v T) {
	for k := 0; k < 100; k++ {
		*p = v
	}
}

func main() {
	x := 0
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			Store(&x, i)
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
