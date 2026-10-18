// phase: 08

// An unbuffered channel hands the value over directly: the send completes only when a receiver
// has taken the value, and the receive happens before the send completes.
package main

import "fmt"

func main() {
	c := make(chan int)
	log := make(chan string, 16)
	go func() {
		log <- "sender: about to send"
		c <- 42
		log <- "sender: send completed"
		close(log)
	}()
	v := <-c
	fmt.Println("main received", v)
	for {
		l, ok := <-log
		if !ok {
			break
		}
		fmt.Println(l)
	}
	fmt.Println(len(c), cap(c))
}

// Output:
// main received 42
// sender: about to send
// sender: send completed
// 0 0
