// phase: 08

// select over named channel types, directional parameters and a channel in a struct field and in a
// slice (the channel expression is evaluated once).
package main

import "fmt"

type Pipe chan int

type bus struct {
	in  chan int
	out chan string
}

func relay(in <-chan int, out chan<- string, n int) {
	for i := 0; i < n; i++ {
		select {
		case v := <-in:
			select {
			case out <- fmt.Sprint("v", v):
			}
		}
	}
}

func main() {
	p := make(Pipe, 1)
	select {
	case p <- 3:
	}
	select {
	case v := <-p:
		fmt.Println(v)
	}
	b := bus{make(chan int, 3), make(chan string, 3)}
	for i := 0; i < 3; i++ {
		b.in <- i
	}
	go relay(b.in, b.out, 3)
	for i := 0; i < 3; i++ {
		select {
		case s := <-b.out:
			fmt.Println(s)
		}
	}
	list := []chan int{make(chan int, 1), make(chan int, 1)}
	idx := 0
	next := func() int { idx++; return idx - 1 }
	select {
	case list[next()] <- 10:
		fmt.Println("sent", idx)
	}
	fmt.Println(len(list[0]), len(list[1]), idx)
}

// Output:
// 3
// v0
// v1
// v2
// sent 1
// 1 0 1
