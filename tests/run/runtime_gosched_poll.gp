// phase: 08

// Gosched in a polling loop lets the goroutine that the loop waits for run. The loop ends when the
// other goroutine has set the flag, whatever the scheduler does.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	flag := make(chan struct{})
	done := make(chan int)
	go func() {
		spins := 0
		for {
			select {
			case <-flag:
				done <- 1
				return
			default:
				spins++
				runtime.Gosched()
			}
		}
	}()
	runtime.Gosched()
	close(flag)
	fmt.Println("goroutine saw the flag:", <-done)
}

// Output:
// goroutine saw the flag: 1
