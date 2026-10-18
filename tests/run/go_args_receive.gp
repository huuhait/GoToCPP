// phase: 08

// The arguments of a go statement are evaluated by the calling goroutine, including receives: the
// value is taken from the channel before the new goroutine starts.
package main

import "fmt"

func show(out chan<- string, a, b int) { out <- fmt.Sprint(a, b) }

func main() {
	in := make(chan int, 4)
	out := make(chan string)
	in <- 1
	in <- 2
	in <- 3
	in <- 4
	go show(out, <-in, <-in)
	fmt.Println(len(in), <-out) // both receives happened already
	go func(a, b int) { out <- fmt.Sprint(a+b, len(in)) }(<-in, <-in)
	fmt.Println(<-out)
}

// Output:
// 2 1 2
// 7 0
