// phase: 08

// Range over a receive-only channel and over a bidirectional one; a send-only parameter is only
// sent to.
package main

import "fmt"

func produce(out chan<- int, n int) {
	for i := 0; i < n; i++ {
		out <- i
	}
	close(out)
}

func consume(in <-chan int) (sum int) {
	for v := range in {
		sum += v
	}
	return
}

func main() {
	c := make(chan int)
	go produce(c, 100)
	fmt.Println(consume(c))

	var ro <-chan int
	b := make(chan int, 3)
	b <- 1
	b <- 2
	close(b)
	ro = b
	for v := range ro {
		fmt.Println(v)
	}
	fmt.Printf("%T %T\n", ro, (chan<- int)(nil))
}

// Output:
// 4950
// 1
// 2
// <-chan int chan<- int
