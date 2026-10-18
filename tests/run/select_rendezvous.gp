// phase: 08

// Two selects meet on an unbuffered channel: one sends and one receives, both blocked in a select
// on more than one channel. Each side also has a case that never happens.
package main

import "fmt"

func main() {
	c := make(chan int)
	never1 := make(chan int)
	never2 := make(chan int)
	done := make(chan int)
	const n = 500
	go func() {
		sum := 0
		for i := 0; i < n; i++ {
			select {
			case v := <-c:
				sum += v
			case <-never1:
				fmt.Println("never1")
			}
		}
		done <- sum
	}()
	for i := 1; i <= n; i++ {
		select {
		case c <- i:
		case never2 <- i:
			fmt.Println("never2")
		}
	}
	fmt.Println(<-done)
}

// Output:
// 125250
