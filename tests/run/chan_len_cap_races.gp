// phase: 08

// len(c) is evaluated after the receive that comes before it in the same statement.
package main

import "fmt"

func main() {
	c := make(chan int, 5)
	for i := 0; i < 5; i++ {
		c <- i
	}
	fmt.Println(len(c), <-c, len(c), <-c, len(c))
	c <- 9
	fmt.Println(<-c, len(c), cap(c), len(c)+cap(c))
	var arr [4]int
	d := make(chan int, 2)
	d <- 1
	fmt.Println(len(arr), len(d), cap(d), len(arr)+len(d))
}

// Output:
// 5 0 4 1 3
// 2 3 5 8
// 4 1 2 5
