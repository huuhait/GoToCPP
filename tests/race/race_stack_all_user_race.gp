// phase: 08
// ExitCode: 66
// Stderr: WARNING: DATA RACE
// Stderr: (?s)WARNING: DATA RACE\n(Write|Read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n.*\nPrevious (write|read) at 0x[0-9a-f]+ by (main goroutine|goroutine \d+):\n
// Stderr: (?m)^Goroutine \d+ \((running|finished)\) created at:$
// Stderr: Found \d+ data race\(s\)
// Stderr: race_stack_all_user_race\.g[op]:(25)\b

// The control of ok_stack_all_parked_churn: a real race of the program is still reported while
// runtime.Stack(buf, true) dumps its goroutines (the dump hides the runtime's records from the
// race detector, nothing of the program's memory). Two goroutines add to a shared counter without
// synchronization, and sleep between the additions, while a third dumps all stacks.
package main

import (
	"fmt"
	"runtime"
	"time"
)

var counter int

func work(done chan bool) {
	for i := 0; i < 100; i++ {
		counter++
		time.Sleep(20 * time.Microsecond)
	}
	done <- true
}

func main() {
	done := make(chan bool)
	go work(done)
	go work(done)
	buf := make([]byte, 1<<16)
	for i := 0; i < 20; i++ {
		runtime.Stack(buf, true)
	}
	<-done
	<-done
	fmt.Println("done")
}

// Output:
// done
