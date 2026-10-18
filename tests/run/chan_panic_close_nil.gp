// phase: 08
// ExitCode: 2
// Stderr: ^panic: close of nil channel\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

func main() {
	var c chan int
	fmt.Println("closing")
	close(c)
}

// Output:
// closing
