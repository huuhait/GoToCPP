// phase: 08

// A goroutine panics, its deferred call wakes main and then blocks for ever. The program does not wait
// for ever for that panic: main returns, and the program ends with exit code 0 (Go's runtime.main
// yields 1000 times for a panic that is running deferred calls, and then exits).
package main

import "fmt"

func main() {
	done := make(chan bool)
	go func() {
		defer func() {
			close(done)
			<-make(chan int) // blocks for ever
		}()
		fmt.Println("goroutine starts")
		panic("never reported")
	}()
	<-done
	fmt.Println("main returns")
}

// Output:
// goroutine starts
// main returns
