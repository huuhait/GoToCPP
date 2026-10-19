// phase: 08

// A receive to the left of a must call happens before the call (step 10.13): Go evaluates the operands of an
// expression with calls and receives in source order, so the receive that the lowering would leave after
// the hoisted call is a wrong order. The twin (testdata/must_twin/must_ctx_order_recv.go) hoists by hand.

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func main() {
	c := make(chan int, 4)
	c <- 1
	c <- 2
	f := func() (int, error) { return <-c * 10, nil }
	x := <-c + f!()
	fmt.Println(x)

	c <- 3
	c <- 4
	c <- 5
	g := func() (int, error) { return <-c, nil }
	y := []int{<-c, g!(), <-c}
	fmt.Println(y[0], y[1], len(c))

	c <- 6
	c <- 7
	z := <-c*100 + g!()
	fmt.Println(z)
}


// Output:
// 21
// 3 4 0
// 607
