// phase: 08

// chan T converts to <-chan T and chan<- T; the three are different types for type switches and
// for interfaces, although they are one channel.
package main

import "fmt"

type RO <-chan int
type WO chan<- int
type Pipe chan string

func kind(x any) string {
	switch x.(type) {
	case chan int:
		return "chan int"
	case <-chan int:
		return "<-chan int"
	case chan<- int:
		return "chan<- int"
	case RO:
		return "RO"
	case WO:
		return "WO"
	case Pipe:
		return "Pipe"
	case chan string:
		return "chan string"
	case chan (<-chan int):
		return "chan (<-chan int)"
	case <-chan <-chan int:
		return "<-chan <-chan int"
	case chan<- chan int:
		return "chan<- chan int"
	}
	return "other"
}

func produce(out chan<- int, n int) {
	for i := 0; i < n; i++ {
		out <- i
	}
	close(out)
}

func consume(in <-chan int) (sum int) {
	for {
		v, ok := <-in
		if !ok {
			return
		}
		sum += v
	}
}

func main() {
	c := make(chan int, 4)
	var r <-chan int = c
	var w chan<- int = c
	fmt.Println(kind(c), "|", kind(r), "|", kind(w))
	fmt.Println(kind(RO(c)), kind(WO(c)), kind(Pipe(nil)), kind(chan string(nil)))
	fmt.Println(kind(make(chan (<-chan int))), kind(make(<-chan (<-chan int))), kind(make(chan<- chan int)), kind(make(chan chan int)))

	go produce(c, 10)
	fmt.Println(consume(c))

	var a, b any = c, r
	fmt.Println(a == b, a == any(c), b == any(r), a == any(w))
	var ro RO = c
	var back <-chan int = ro
	fmt.Println(back == r, ro == RO(c))
	p := make(Pipe, 1)
	p <- "named"
	fmt.Println(<-p, len(p), cap(p))
	var plain chan string = p
	plain <- "plain"
	fmt.Println(<-p)
}

// Output:
// chan int | <-chan int | chan<- int
// RO WO Pipe chan string
// chan (<-chan int) <-chan <-chan int chan<- chan int other
// 45
// false true true false
// true true
// named 0 1
// plain
