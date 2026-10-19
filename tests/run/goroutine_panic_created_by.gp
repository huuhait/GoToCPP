// phase: 08
// ExitCode: 2
// Stderr: ^panic: boom in a goroutine\n\ngoroutine \d+ \[running\]:\n(?:.+\n\t.+\n)+created by main\.main in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n$

// The trace of a panic in a goroutine ends with Go's "created by" line: the function with the go
// statement, the id of the goroutine that ran it, and the line of the go statement.
package main

import "fmt"

func main() {
	fmt.Println("starting")
	go func() {
		panic("boom in a goroutine")
	}()
	<-make(chan int)
}

// Output:
// starting
