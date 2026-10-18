// phase: 08

// All the channel expressions and the values to send are evaluated once, in source order, before
// the select communicates, also for the cases that are not chosen.
package main

import "fmt"

var log []string

func ch(name string, c chan int) chan int {
	log = append(log, "chan "+name)
	return c
}

func val(name string, v int) int {
	log = append(log, "val "+name)
	return v
}

func main() {
	a := make(chan int, 1)
	b := make(chan int, 1)
	c := make(chan int, 1)
	a <- 0 // a and c are full: a send to them is not ready
	c <- 0
	b <- 1
	select {
	case ch("a", a) <- val("one", 1):
		fmt.Println("sent a")
	case v := <-ch("b", b):
		fmt.Println("recv b", v)
	case ch("c", c) <- val("two", 2):
		fmt.Println("sent c")
	case <-ch("d", nil):
		fmt.Println("never")
	}
	fmt.Println(log)

	// the same channel expression is evaluated once per statement, not once per poll
	log = nil
	n := 0
	g := make(chan int)
	f := func() chan int { n++; return g }
	select {
	case <-f():
	case f() <- n:
	default:
	}
	fmt.Println(n)
}

// Output:
// recv b 1
// [chan a val one chan b chan c val two chan d]
// 2
