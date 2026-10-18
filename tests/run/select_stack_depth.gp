// phase: 08

// select at the bottom of a deep recursion, and in a recursion that sends through a channel at every
// level.
package main

import "fmt"

func descend(n int, c chan int) int {
	if n == 0 {
		select {
		case v := <-c:
			return v
		default:
			return -1
		}
	}
	return descend(n-1, c) + 1
}

func chain(n int, in chan int, out chan int) {
	if n == 0 {
		select {
		case v := <-in:
			out <- v
		}
		return
	}
	mid := make(chan int, 1)
	go chain(n-1, in, mid)
	select {
	case v := <-mid:
		out <- v + 1
	}
}

func main() {
	c := make(chan int, 1)
	c <- 7
	fmt.Println(descend(5000, c))
	fmt.Println(descend(5000, c))
	in := make(chan int, 1)
	out := make(chan int, 1)
	in <- 100
	go chain(200, in, out)
	fmt.Println(<-out)
}

// Output:
// 5007
// 4999
// 300
