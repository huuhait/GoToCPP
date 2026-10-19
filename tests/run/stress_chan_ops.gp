// phase: 09

// Stress: channel operations (GOCPP_STRESS=1: about 4 million; default: about 40 thousand). A buffered
// producer/consumer pair, an unbuffered ping-pong, a three-stage pipeline, a select over three channels.
// Run it big with: GOCPP_STRESS=1 go test ./tests -run 'Run/stress_chan_ops'
package main

import (
	"fmt"
	"os"
)

func scale(small, big int) int {
	if os.Getenv("GOCPP_STRESS") == "1" {
		return big
	}
	return small
}

func main() {
	n := scale(10000, 1000000)

	// buffered, one producer and one consumer
	buf := make(chan int, 256)
	go func() {
		for i := 1; i <= n; i++ {
			buf <- i
		}
		close(buf)
	}()
	sum := 0
	for v := range buf {
		sum += v
	}
	fmt.Println("buffered:", sum == n*(n+1)/2)

	// unbuffered ping-pong: 2 hand-offs per round
	ping, pong := make(chan int), make(chan int)
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
	fmt.Println("ping-pong:", v == n/10)

	// pipeline: generate -> double -> add one
	gen := make(chan int, 64)
	dbl := make(chan int, 64)
	inc := make(chan int, 64)
	go func() {
		for i := 0; i < n; i++ {
			gen <- i
		}
		close(gen)
	}()
	go func() {
		for v := range gen {
			dbl <- v * 2
		}
		close(dbl)
	}()
	go func() {
		for v := range dbl {
			inc <- v + 1
		}
		close(inc)
	}()
	psum := 0
	for v := range inc {
		psum += v
	}
	fmt.Println("pipeline:", psum == n*(n-1)+n)

	// select over three producers; the receiver counts every value once
	a, b, c := make(chan int, 16), make(chan int, 16), make(chan int)
	feed := func(ch chan int, start int) {
		for i := 0; i < n/3; i++ {
			ch <- start + i
		}
		close(ch)
	}
	go feed(a, 0)
	go feed(b, n)
	go feed(c, 2*n)
	count, seen := 0, 0
	for a != nil || b != nil || c != nil {
		select {
		case x, ok := <-a:
			if !ok {
				a = nil
			} else {
				count++
				seen += x / n
			}
		case x, ok := <-b:
			if !ok {
				b = nil
			} else {
				count++
				seen += x / n
			}
		case x, ok := <-c:
			if !ok {
				c = nil
			} else {
				count++
				seen += x / n
			}
		}
	}
	fmt.Println("select:", count == 3*(n/3), seen == (n/3)*(0+1+2))
}

// Output:
// buffered: true
// ping-pong: true
// pipeline: true
// select: true true
