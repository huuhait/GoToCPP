// phase: 08

// Directional channel types in struct fields, parameters, results, slices and func types: a
// bidirectional channel is assignable to all of them.
package main

import "fmt"

type Conn struct {
	In  <-chan string
	Out chan<- string
}

func pipe() (<-chan string, chan<- string) {
	c := make(chan string, 4)
	return c, c
}

func echo(c Conn, n int) {
	for i := 0; i < n; i++ {
		c.Out <- "echo " + <-c.In
	}
}

type Handler func(in <-chan int, out chan<- int)

func main() {
	a := make(chan string, 4)
	b := make(chan string, 4)
	go echo(Conn{In: a, Out: b}, 2)
	a <- "one"
	a <- "two"
	fmt.Println(<-b, <-b)

	r, w := pipe()
	w <- "through pipe"
	fmt.Println(<-r)

	var h Handler = func(in <-chan int, out chan<- int) { out <- <-in * 3 }
	in, out := make(chan int, 1), make(chan int, 1)
	in <- 14
	h(in, out)
	fmt.Println(<-out)

	var recvs []<-chan int
	var sends []chan<- int
	for i := 0; i < 3; i++ {
		c := make(chan int, 1)
		recvs = append(recvs, c)
		sends = append(sends, c)
	}
	for i, s := range sends {
		s <- i * i
	}
	for _, r := range recvs {
		fmt.Print(<-r, ";")
	}
	fmt.Println()
	m := map[string]chan<- int{"x": out}
	m["x"] <- 5
	fmt.Println(<-out, len(m))
}

// Output:
// echo one echo two
// through pipe
// 42
// 0;1;4;
// 5 1
