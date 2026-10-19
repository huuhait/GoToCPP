// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\n
// Stderr: \ngoroutine \d+ \[chan receive\]:\n

// A goroutine recovers a panic and then waits in the same deferred call for main, which waits for it.
package main

import "fmt"

func main() {
	done := make(chan int)
	go func() {
		defer func() {
			recover()
			fmt.Println("recovered, waiting")
			<-done
		}()
		panic("x")
	}()
	<-done
}

// Output:
// recovered, waiting
