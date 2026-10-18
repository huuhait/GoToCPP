// phase: 08
// ExitCode: 2
// Stderr: ^panic: simple\n\ngoroutine \d+ \[running\]:\n

// A goroutine panics, and its deferred call closes a channel that main waits for. Main returns
// at once, but the program still ends with the panic (exit code 2): Go lets a goroutine that
// runs the deferred calls of a panic finish before main's return ends the program.
package main

import "fmt"

func main() {
	done := make(chan bool)
	go func() {
		defer close(done)
		fmt.Println("goroutine starts")
		panic("simple")
	}()
	<-done
}

// Output:
// goroutine starts
