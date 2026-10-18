// phase: 08

// A goroutine blocked in a select with a send case panics when that channel is closed, and the
// panic can be recovered inside the goroutine.
package main

import "fmt"

func main() {
	c := make(chan int)
	other := make(chan int)
	res := make(chan string)
	blocked := make(chan bool)
	go func() {
		defer func() {
			res <- fmt.Sprint("recovered: ", recover())
		}()
		blocked <- true
		select {
		case c <- 1:
		case <-other:
		}
	}()
	<-blocked
	// Let the goroutine reach the select, then close. If it has not yet, it panics at the first poll.
	close(c)
	fmt.Println(<-res)
}

// Output:
// recovered: send on closed channel
