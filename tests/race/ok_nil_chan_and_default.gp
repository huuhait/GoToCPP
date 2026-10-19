// phase: 08

// Select with a default and a nil channel, in one goroutine, with a channel handoff.
package main

import "fmt"

func main() {
	var nilc chan int
	data := make(chan int, 1)
	x := 0
	go func() {
		x = 5
		data <- 1
	}()
	<-data
	select {
	case <-nilc:
	default:
		x++
	}
	fmt.Println(x)
}

// Output:
// 6
