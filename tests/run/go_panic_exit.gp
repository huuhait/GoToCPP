// phase: 08
// ExitCode: 2
// Stderr: ^panic: boom\n\ngoroutine \d+ \[running\]:\n

// An unrecovered panic in a goroutine ends the whole program with exit code 2, while main waits.
package main

import "fmt"

func main() {
	fmt.Println("main starts")
	block := make(chan int)
	go func() {
		defer fmt.Println("deferred in the goroutine runs first")
		panic("boom")
	}()
	<-block
	fmt.Println("not reached")
}

// Output:
// main starts
// deferred in the goroutine runs first
