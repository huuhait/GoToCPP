// phase: 08

// Generators: a function returns a channel (or a closure over one) that yields a sequence.
package main

import "fmt"

func fib(n int) <-chan int {
	c := make(chan int)
	go func() {
		a, b := 0, 1
		for i := 0; i < n; i++ {
			c <- a
			a, b = b, a+b
		}
		close(c)
	}()
	return c
}

func counter(start int) func() int {
	c := make(chan int)
	go func() {
		for i := start; ; i++ {
			c <- i
		}
	}()
	return func() int { return <-c }
}

func take(c <-chan int, n int) []int {
	var out []int
	for i := 0; i < n; i++ {
		v, ok := <-c
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out
}

func main() {
	fmt.Println(take(fib(15), 100))
	next := counter(10)
	fmt.Println(next(), next(), next())
	other := counter(100)
	fmt.Println(other(), next(), other())
}

// Output:
// [0 1 1 2 3 5 8 13 21 34 55 89 144 233 377]
// 10 11 12
// 100 13 101
