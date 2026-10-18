// phase: 08

// The channel of a case is evaluated when the select starts, not when it polls: a later case's call
// that changes the variable does not change the channel of the earlier case.
package main

import "fmt"

func main() {
	c := make(chan int, 1)
	d := make(chan int, 1)
	c <- 1
	d <- 2
	out := make(chan int) // nobody receives
	f := func() int {
		c = d // c is captured and changed
		return 0
	}
	select {
	case v := <-c:
		fmt.Println("recv", v)
	case out <- f():
		fmt.Println("never")
	}
	fmt.Println(len(c), len(d))

	// the same with a field: the struct is changed by the call
	type holder struct{ ch chan int }
	a, b := make(chan int, 1), make(chan int, 1)
	a <- 10
	b <- 20
	h := &holder{a}
	g := func() int {
		h.ch = b
		return 0
	}
	select {
	case v := <-h.ch:
		fmt.Println("field", v)
	case out <- g():
		fmt.Println("never")
	}
	fmt.Println(len(a), len(b), h.ch == b)

	// a send case whose own value calls a function that changes its channel: the channel was read first
	e1, e2 := make(chan int, 1), make(chan int, 1)
	orig := e1
	g2 := func() int {
		e1 = e2
		return 7
	}
	select {
	case e1 <- g2():
		fmt.Println("sent")
	}
	fmt.Println(len(orig), len(e1), len(e2))
}

// Output:
// recv 1
// 1 1
// field 10
// 0 1 true
// sent
// 1 0 0
