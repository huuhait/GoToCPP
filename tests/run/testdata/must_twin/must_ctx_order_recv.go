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
	t1 := <-c
	t2, err := f()
	if err != nil {
		panic(err)
	}
	x := t1 + t2
	fmt.Println(x)

	c <- 3
	c <- 4
	c <- 5
	g := func() (int, error) { return <-c, nil }
	e0 := <-c
	e1, err := g()
	if err != nil {
		panic(err)
	}
	e2 := <-c
	y := []int{e0, e1, e2}
	fmt.Println(y[0], y[1], len(c))

	c <- 6
	c <- 7
	r := <-c
	w, err := g()
	if err != nil {
		panic(err)
	}
	z := r*100 + w
	fmt.Println(z)
}
