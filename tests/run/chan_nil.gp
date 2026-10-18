// phase: 08

// A nil channel: len and cap are 0, comparing with nil works, and sends and receives block for
// ever. Another goroutine keeps working while the first one is blocked on a nil channel.
package main

import "fmt"

func main() {
	var c chan int
	fmt.Println(c == nil, len(c), cap(c))
	blocked := make(chan string)
	go func() {
		var nc chan int
		nc <- 1 // blocks for ever
		blocked <- "never"
	}()
	go func() {
		var nc chan string
		fmt.Println(<-nc) // blocks for ever
		blocked <- "never"
	}()
	work := make(chan int)
	go func() {
		s := 0
		for i := 1; i <= 100; i++ {
			s += i
		}
		work <- s
	}()
	fmt.Println(<-work)
	c = make(chan int, 1)
	fmt.Println(c != nil)
	c = nil
	fmt.Println(c == nil)
}

// Output:
// true 0 0
// 5050
// true
// true
