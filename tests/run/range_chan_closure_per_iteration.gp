// phase: 08

// Go 1.22: each iteration has its own variable, so closures that capture it (and pointers to it)
// see the value of their iteration.
package main

import "fmt"

func main() {
	c := make(chan int, 4)
	for i := 1; i <= 4; i++ {
		c <- i * 10
	}
	close(c)
	var fs []func() int
	var ps []*int
	for v := range c {
		fs = append(fs, func() int { v++; return v })
		ps = append(ps, &v)
	}
	for i, f := range fs {
		a, b := f(), f()
		fmt.Println(a, b, *ps[i])
	}
	// the variable can be changed in the body without affecting the loop
	d := make(chan int, 3)
	d <- 1
	d <- 2
	d <- 3
	close(d)
	for v := range d {
		v *= 100
		fmt.Println(v)
	}
	// a goroutine per value that captures the loop variable
	e := make(chan int, 5)
	for i := 0; i < 5; i++ {
		e <- i
	}
	close(e)
	res := make(chan int, 5)
	for v := range e {
		go func() { res <- v * v }()
	}
	total := 0
	for i := 0; i < 5; i++ {
		total += <-res
	}
	fmt.Println(total)
}

// Output:
// 11 12 12
// 21 22 22
// 31 32 32
// 41 42 42
// 100
// 200
// 300
// 30
