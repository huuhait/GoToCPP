package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() { c.n++ }

func newCounter() *Counter {
	c := Counter{n: 1} // its address is taken: boxed
	return &c
}

func main() {
	var local Counter // a pointer method is called on it: boxed
	local.Inc()
	x := 41 // nothing sees x after the function: it stays on the stack
	x++
	p := newCounter()
	p.Inc()
	fmt.Println(local.n, x, p.n)
}
