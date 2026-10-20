// bench: concrete_iface
// category: dispatch
// size: 40000000
// smoke: 100000
// twin: no
// about: a stable concrete interface receiver in a dependent integer update loop
package main

import "fmt"

type counter struct{ n uint64 }
type stepper interface{ step(uint64) uint64 }

func (c *counter) step(x uint64) uint64 {
	c.n = c.n*1664525 + x + 1013904223
	return c.n
}

func main() {
	p := &counter{n: 7}
	var s stepper = p
	var sum uint64
	for i := uint64(0); i < uint64(benchN); i++ {
		sum += s.step(i)
	}
	fmt.Println(sum, p.n)
}
