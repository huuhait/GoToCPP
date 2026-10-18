// phase: 08

// defer with channels: defer close(c), a deferred send, and a deferred receive from a closed channel.
package main

import "fmt"

func producer(n int) <-chan int {
	c := make(chan int)
	go func() {
		defer close(c)
		for i := 0; i < n; i++ {
			c <- i * i
		}
	}()
	return c
}

func guarded(done chan<- string) {
	defer func() { done <- "guarded finished" }()
	defer fmt.Println("deferred print")
	panicky := func() {
		defer func() { recover() }()
		var c chan int
		close(c)
	}
	panicky()
}

func main() {
	c := producer(5)
	for {
		v, ok := <-c
		if !ok {
			break
		}
		fmt.Print(v, ",")
	}
	fmt.Println()
	done := make(chan string)
	go guarded(done)
	fmt.Println(<-done)

	buf := make(chan int, 1)
	func() {
		defer func() { fmt.Println("deferred receive:", <-buf) }()
		buf <- 42
	}()
}

// Output:
// 0,1,4,9,16,
// deferred print
// guarded finished
// deferred receive: 42
