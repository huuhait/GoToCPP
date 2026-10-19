// phase: 08

// A goroutine waits for main while main waits for a goroutine that sleeps: one of them always
// has something that wakes the other.
package main

import (
	"fmt"
	"time"
)

func main() {
	ping, pong := make(chan int), make(chan int)
	go func() {
		for v := range ping {
			time.Sleep(time.Millisecond)
			pong <- v * 2
		}
		close(pong)
	}()
	for i := 1; i <= 5; i++ {
		ping <- i
		fmt.Println(<-pong)
	}
	close(ping)
	_, ok := <-pong
	fmt.Println(ok)
}

// Output:
// 2
// 4
// 6
// 8
// 10
// false
