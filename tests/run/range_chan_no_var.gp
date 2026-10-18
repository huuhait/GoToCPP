// phase: 08

// for range ch counts the values without a variable; the values are dropped.
package main

import "fmt"

type big struct {
	data [64]int
	name string
}

func main() {
	c := make(chan big, 10)
	for i := 0; i < 10; i++ {
		c <- big{name: fmt.Sprint("n", i)}
	}
	close(c)
	n := 0
	for range c {
		n++
	}
	fmt.Println(n)

	d := make(chan int)
	go func() {
		for i := 0; i < 1000; i++ {
			d <- i
		}
		close(d)
	}()
	m := 0
	for _ = range d {
		m++
	}
	fmt.Println(m)
}

// Output:
// 10
// 1000
