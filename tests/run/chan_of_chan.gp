// phase: 08

// A channel of channels: request/reply, where each request carries its own reply channel.
package main

import "fmt"

type request struct {
	n     int
	reply chan int
}

func server(reqs <-chan request) {
	for {
		r, ok := <-reqs
		if !ok {
			return
		}
		r.reply <- r.n * r.n
	}
}

func main() {
	reqs := make(chan request)
	go server(reqs)
	for i := 1; i <= 5; i++ {
		reply := make(chan int)
		reqs <- request{i, reply}
		fmt.Print(<-reply, ",")
	}
	fmt.Println()
	close(reqs)

	cc := make(chan chan string, 1)
	inner := make(chan string, 1)
	cc <- inner
	got := <-cc
	got <- "through two levels"
	fmt.Println(<-inner)

	chain := make(chan (<-chan int), 3)
	for i := 0; i < 3; i++ {
		c := make(chan int, 1)
		c <- i * 100
		chain <- c
	}
	close(chain)
	for {
		c, ok := <-chain
		if !ok {
			break
		}
		fmt.Print(<-c, ",")
	}
	fmt.Println()
}

// Output:
// 1,4,9,16,25,
// through two levels
// 0,100,200,
