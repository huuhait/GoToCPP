// phase: 08

// After close, receives return the buffered values first, then zero values with ok == false.
package main

import "fmt"

func main() {
	c := make(chan int, 3)
	c <- 1
	c <- 2
	close(c)
	for i := 0; i < 4; i++ {
		v, ok := <-c
		fmt.Println(v, ok, len(c), cap(c))
	}
	u := make(chan string)
	close(u)
	s, ok := <-u
	fmt.Printf("%q %v\n", s, ok)
	type pair struct {
		a int
		b string
	}
	p := make(chan pair, 1)
	close(p)
	z, ok := <-p
	fmt.Println(z, ok)
	var np chan *int = make(chan *int)
	close(np)
	ptr, ok := <-np
	fmt.Println(ptr == nil, ok)
	sl := make(chan []int)
	close(sl)
	e, ok := <-sl
	fmt.Println(e == nil, len(e), ok)
}

// Output:
// 1 true 1 3
// 2 true 0 3
// 0 false 0 3
// 0 false 0 3
// "" false
// {0 } false
// true false
// true 0 false
