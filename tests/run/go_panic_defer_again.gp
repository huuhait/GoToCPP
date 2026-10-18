// phase: 08
// ExitCode: 2
// Stderr: ^panic: first \[recovered\]\n\tpanic: again\n\ngoroutine \d+ \[running\]:\n

// A deferred call recovers a panic and panics again; a deferred call below it wakes main.
package main

import "fmt"

func main() {
	done := make(chan bool)
	go func() {
		defer close(done)
		defer func() {
			fmt.Println("recovered:", recover())
			panic("again")
		}()
		panic("first")
	}()
	<-done
}

// Output:
// recovered: first
