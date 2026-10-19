// bench: chan_pingpong_buf
// category: runtime
// size: 4000000
// smoke: 10000
// twin: no
// about: a producer sends ints through a buffered channel (capacity 128) to a consumer, then a ping-pong over capacity-1 channels
package main

import "fmt"

func main() {
	n := benchN
	ch := make(chan int, 128)
	go func() {
		for i := 0; i < n; i++ {
			ch <- i
		}
		close(ch)
	}()
	sum := 0
	for v := range ch {
		sum += v
	}
	ping := make(chan int, 1)
	pong := make(chan int, 1)
	go func() {
		for v := range ping {
			pong <- v + 1
		}
		close(pong)
	}()
	v := 0
	for i := 0; i < n/10; i++ {
		ping <- v
		v = <-pong
	}
	close(ping)
	fmt.Println(sum, v)
}
