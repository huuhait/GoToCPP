// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: (?m)^  main\.\S+\(\)$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_select_shared\.g[op]:(25|29|36)\b

// Two goroutines that communicate through select still write a variable they share.
package main

import "fmt"

var last int

func main() {
	a := make(chan int)
	b := make(chan int)
	done := make(chan bool)
	go func() {
		select {
		case v := <-a:
			for k := 0; k < 100; k++ {
				last = v
			}
		case v := <-b:
			for k := 0; k < 100; k++ {
				last = v
			}
		}
		done <- true
	}()
	go func() {
		for k := 0; k < 100; k++ {
			last = 99
		}
		done <- true
	}()
	a <- 1
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
