// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[select\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A select whose cases are all nil channels.
package main

import "fmt"

func main() {
	var a chan int
	var b chan string
	fmt.Println("selecting on nil channels")
	select {
	case <-a:
	case b <- "x":
	}
}

// Output:
// selecting on nil channels
