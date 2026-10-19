// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\n
// Stderr: \ngoroutine \d+ \[chan receive\]:\n(?:.+\n\t.+\n)+created by main\.main in goroutine 1\n

// 2000 goroutines wait for ever: the report lists every one of them, and comes at once (the stacks
// of the parked threads are captured by the threads themselves, and the names are resolved once).
package main

import "fmt"

func main() {
	c := make(chan int)
	for i := 0; i < 2000; i++ {
		go func() { <-c }()
	}
	fmt.Println("started")
	<-c
}

// Output:
// started
