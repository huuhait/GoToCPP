// phase: 08

// The range expression is evaluated once, before the loop; the channel variable can be reassigned in
// the body without changing the loop.
package main

import "fmt"

var calls int

func source() chan int {
	calls++
	c := make(chan int, 3)
	c <- 1
	c <- 2
	c <- 3
	close(c)
	return c
}

func main() {
	for v := range source() {
		fmt.Println(v)
	}
	fmt.Println("calls", calls)

	a := make(chan int, 2)
	a <- 10
	a <- 20
	close(a)
	b := make(chan int, 1)
	b <- 99
	close(b)
	ch := a
	for v := range ch {
		ch = b // does not change what is ranged over
		fmt.Println(v)
	}
	fmt.Println(len(b))

	// len and cap while looping
	d := make(chan int, 4)
	for i := 0; i < 4; i++ {
		d <- i
	}
	close(d)
	for v := range d {
		fmt.Println(v, len(d), cap(d))
	}
}

// Output:
// 1
// 2
// 3
// calls 1
// 10
// 20
// 1
// 0 3 4
// 1 2 4
// 2 1 4
// 3 0 4
