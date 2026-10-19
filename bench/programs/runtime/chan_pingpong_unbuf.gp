// bench: chan_pingpong_unbuf
// category: runtime
// size: 800000
// smoke: 2000
// twin: yes
// about: two goroutines pass a counter back and forth over unbuffered channels (one round trip = two hand-offs)
package main

import "fmt"

func main() {
	ping := make(chan int)
	pong := make(chan int)
	go func() {
		for v := range ping {
			pong <- v + 1
		}
		close(pong)
	}()
	v := 0
	for i := 0; i < benchN; i++ {
		ping <- v
		v = <-pong
	}
	close(ping)
	fmt.Println(v)
}
