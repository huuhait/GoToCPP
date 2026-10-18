// phase: 08

// A send copies the value: changing the sender's variable afterwards does not change what is in the
// channel, and a receive makes another copy.
package main

import "fmt"

type Big struct {
	name string
	data [100]int
}

func main() {
	c := make(chan Big, 2)
	var b Big
	b.name = "first"
	b.data[0], b.data[99] = 1, 99
	c <- b
	b.name = "second"
	b.data[0] = -1
	c <- b
	r1, r2 := <-c, <-c
	fmt.Println(r1.name, r1.data[0], r1.data[99], r2.name, r2.data[0], r2.data[99])
	r1.data[5] = 5
	fmt.Println(r2.data[5], b.data[5])

	u := make(chan Big)
	go func() {
		var local Big
		local.name = "from goroutine"
		local.data[10] = 10
		u <- local
		local.data[10] = 11 // after the send: invisible to the receiver
		u <- local
	}()
	x, y := <-u, <-u
	fmt.Println(x.name, x.data[10], y.data[10])
}

// Output:
// first 1 99 second -1 99
// 0 0
// from goroutine 10 11
