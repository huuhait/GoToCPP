// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\n

// A panic that was recovered leaves nothing behind that keeps the detector quiet.
package main

import "fmt"

func recovers() {
	defer func() { fmt.Println("recovered:", recover()) }()
	panic("gone")
}

func main() {
	recovers()
	go func() {
		defer func() { recover() }()
		panic("also gone")
	}()
	<-make(chan int)
}

// Output:
// recovered: gone
