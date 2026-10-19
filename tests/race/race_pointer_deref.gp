// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_pointer_deref\.g[op]:(20)\b

// Two goroutines write through the same pointer.
package main

import "fmt"

func main() {
	p := new(int)
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func() {
			*p += 1
			done <- true
		}()
	}
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
