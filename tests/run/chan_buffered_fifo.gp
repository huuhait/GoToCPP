// phase: 08

// A buffered channel is a FIFO queue; len and cap track it.
package main

import "fmt"

func main() {
	c := make(chan string, 3)
	fmt.Println(len(c), cap(c))
	c <- "a"
	c <- "b"
	fmt.Println(len(c), cap(c))
	c <- "c"
	fmt.Println(len(c))
	fmt.Println(<-c, <-c)
	fmt.Println(len(c))
	c <- "d"
	c <- "e"
	fmt.Println(len(c), <-c, <-c, <-c, len(c))
}

// Output:
// 0 3
// 2 3
// 3
// a b
// 1
// 3 c d e 0
