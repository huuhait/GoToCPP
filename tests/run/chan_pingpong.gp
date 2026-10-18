// phase: 08

// Two goroutines pass a counter back and forth through two unbuffered channels, 20000 times.
package main

import "fmt"

func main() {
	ping, pong := make(chan int), make(chan int)
	go func() {
		for {
			v, ok := <-ping
			if !ok {
				close(pong)
				return
			}
			pong <- v + 1
		}
	}()
	v := 0
	for i := 0; i < 20000; i++ {
		ping <- v
		v = <-pong
	}
	close(ping)
	_, ok := <-pong
	fmt.Println(v, ok)
}

// Output:
// 20000 false
