// phase: 08

// A sender blocks when the buffer is full and goes on when a receiver makes room. The order of the
// two goroutines' prints is fixed by the channels.
package main

import "fmt"

func main() {
	c := make(chan int, 2)
	step := make(chan string)
	go func() {
		for i := 1; i <= 4; i++ {
			c <- i
			step <- fmt.Sprint("sent ", i)
		}
		close(step)
	}()
	fmt.Println(<-step) // (len(c) is 1 or 2 here: the sender may have put the next value in already)
	fmt.Println(<-step, len(c))
	// the buffer is full now; the third send blocks until we receive
	fmt.Println("received", <-c)
	fmt.Println(<-step)
	fmt.Println("received", <-c)
	fmt.Println(<-step)
	fmt.Println("received", <-c, <-c)
	_, ok := <-step
	fmt.Println(ok, len(c))
}

// Output:
// sent 1
// sent 2 2
// received 1
// sent 3
// received 2
// sent 4
// received 3 4
// false 0
