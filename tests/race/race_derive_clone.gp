// phase: 10
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: Found \d+ data race\(s\)
// Stderr: race_derive_clone\.g[op]:(1[0-9]|2[0-9])\b

// The derived Clone reads the slice that another goroutine writes: the race is found in the code that
// #Derive wrote, which the report places at the #Derive line (step 10.10).
package main

import "fmt"

#Derive(Clone)
type Table struct {
	Rows []int
}

func main() {
	t := Table{Rows: make([]int, 8)}
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			t.Rows[i%8] = i
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = t.Clone()
		}
		done <- true
	}()
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
