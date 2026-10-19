// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive \(nil chan\)\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n\ngoroutine \d+ \[chan send \(nil chan\)\]:\n(?s:.*)created by main\.main in goroutine 1\n

// The receive from and the send to a nil channel wait for ever, each with its own reason.
package main

import "fmt"

func main() {
	var c chan int
	go func() { c <- 1 }()
	fmt.Println("receiving from nil")
	<-c
}

// Output:
// receiving from nil
