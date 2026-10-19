// phase: 08
// ExitCode: 2
// Stderr: (?s)^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[sync\.WaitGroup\.Wait\]:\n
// Stderr: (?s)\ngoroutine \d+ \[chan receive\]:\n.*created by main\.main in goroutine 1\n
// Stderr: (?s)\ngoroutine \d+ \[chan send\]:\n.*created by main\.main in goroutine 1\n

// Main waits for a WaitGroup whose goroutines are stuck: the report lists every goroutine, in
// the order of their ids, each with its reason, its stack and "created by". (Go's wait reason for
// the WaitGroup is "sync.WaitGroup.Wait", checked below.)
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	in := make(chan int)
	out := make(chan int)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-in
	}()
	go func() {
		defer wg.Done()
		out <- 1
	}()
	fmt.Println("waiting")
	wg.Wait()
}

// Output:
// waiting
