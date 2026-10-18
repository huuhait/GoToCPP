// phase: 08

// for v := range ch receives until the channel is closed and drained.
package main

import "fmt"

func main() {
	c := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		c <- i * i
	}
	close(c)
	// a closed buffered channel gives its buffer first
	sum := 0
	for v := range c {
		sum += v
		fmt.Println("got", v)
	}
	fmt.Println(sum, len(c))

	u := make(chan string)
	go func() {
		for _, s := range []string{"x", "y", "z"} {
			u <- s
		}
		close(u)
	}()
	for s := range u {
		fmt.Println(s)
	}
	// ranging over a closed, empty channel ends at once
	n := 0
	for range u {
		n++
	}
	fmt.Println("after", n)
}

// Output:
// got 1
// got 4
// got 9
// got 16
// got 25
// 55 0
// x
// y
// z
// after 0
