// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\n

// A panic whose deferred call waits for ever is a deadlock: Go reports it (the panic message is
// never printed, the program dies of the deadlock first).
package main

import "fmt"

func main() {
	defer func() {
		fmt.Println("the deferred call waits")
		c := make(chan int)
		<-c
	}()
	panic("boom")
}

// Output:
// the deferred call waits
