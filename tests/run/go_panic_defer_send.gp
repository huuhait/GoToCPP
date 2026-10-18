// phase: 08
// ExitCode: 2
// Stderr: ^panic: assignment to entry in nil map\n\ngoroutine \d+ \[running\]:\n

// The same with a deferred send (the shape of `defer wg.Done()`), and a runtime panic.
package main

import "fmt"

func main() {
	done := make(chan bool)
	go func() {
		defer func() { done <- true }()
		var m map[int]int
		fmt.Println("storing")
		m[1] = 1
	}()
	<-done
}

// Output:
// storing
