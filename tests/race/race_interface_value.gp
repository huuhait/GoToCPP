// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_interface_value\.g[op]:(21|27)\b

// Two goroutines assign different dynamic types to an interface variable.
package main

import "fmt"

var v any

func main() {
	done := make(chan bool)
	go func() {
		for k := 0; k < 100; k++ {
			v = 1
		}
		done <- true
	}()
	go func() {
		for k := 0; k < 100; k++ {
			v = "one"
		}
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
