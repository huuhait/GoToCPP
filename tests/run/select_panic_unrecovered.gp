// phase: 08
// ExitCode: 2
// Stderr: ^panic: boom 7\n\ngoroutine 1 \[running\]:\n

// An unrecovered panic in a select case body ends the program with code 2.
package main

import "fmt"

func main() {
	c := make(chan int, 1)
	c <- 7
	select {
	case v := <-c:
		fmt.Println("got", v)
		panic(fmt.Sprint("boom ", v))
	}
}

// Output:
// got 7
