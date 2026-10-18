// phase: 08

// A million values through a buffered channel, and half a million through an unbuffered one.
package main

import "fmt"

func producer(n int, out chan<- int) {
	for i := 1; i <= n; i++ {
		out <- i
	}
	close(out)
}

func total(in <-chan int) int {
	s := 0
	for {
		v, ok := <-in
		if !ok {
			return s
		}
		s += v
	}
}

func main() {
	buffered := make(chan int, 128)
	go producer(1000000, buffered)
	fmt.Println(total(buffered))
	unbuffered := make(chan int)
	go producer(100000, unbuffered)
	fmt.Println(total(unbuffered))
}

// Output:
// 500000500000
// 5000050000
