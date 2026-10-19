// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_string_var\.g[op]:(21|27)\b

// Two goroutines assign a string variable.
package main

import "fmt"

var name string

func main() {
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			name = "first"
		}
		done <- true
	}()
	go func() {
		for k := 0; k < 100; k++ {
			name = "second"
		}
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
