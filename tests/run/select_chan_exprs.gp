// phase: 08

// Channel expressions in cases: a method call, a field, a map lookup (a missing key gives a nil
// channel that is never ready), a slice element, a function result; each is evaluated once.
package main

import "fmt"

type hub struct {
	in   chan int
	outs map[string]chan int
	list []chan int
	calls int
}

func (h *hub) pick() chan int {
	h.calls++
	return h.in
}

func mk() chan int {
	c := make(chan int, 1)
	c <- 77
	return c
}

func main() {
	h := &hub{in: make(chan int, 1), outs: map[string]chan int{"a": make(chan int, 1)}}
	h.list = []chan int{make(chan int, 1), make(chan int, 1)}
	h.in <- 1
	select {
	case v := <-h.pick():
		fmt.Println("pick", v, h.calls)
	}
	select {
	case v := <-h.outs["missing"]:
		fmt.Println("never", v)
	case h.outs["a"] <- 5:
		fmt.Println("sent to a", len(h.outs["a"]))
	}
	select {
	case v := <-h.list[1]:
		fmt.Println("never", v)
	case h.list[1] <- 9:
		fmt.Println("sent to list[1]", len(h.list[0]), len(h.list[1]))
	}
	select {
	case v := <-mk():
		fmt.Println("mk", v)
	}
	// a channel received from a channel
	cc := make(chan chan int, 1)
	cc <- mk()
	select {
	case v := <-<-cc:
		fmt.Println("nested", v)
	}
}

// Output:
// pick 1 1
// sent to a 1
// sent to list[1] 0 1
// mk 77
// nested 77
