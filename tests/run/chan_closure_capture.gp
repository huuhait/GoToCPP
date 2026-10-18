// phase: 08

// Closures that capture channels and variables, run as goroutines.
package main

import "fmt"

func adder(in <-chan int, out chan<- int, k int) {
	for {
		v, ok := <-in
		if !ok {
			close(out)
			return
		}
		out <- v + k
	}
}

func main() {
	src := make(chan int)
	c := src
	for k := 1; k <= 4; k++ {
		next := make(chan int)
		go adder(c, next, k)
		c = next
	}
	go func() {
		for i := 0; i < 5; i++ {
			src <- i * 100
		}
		close(src)
	}()
	for {
		v, ok := <-c
		if !ok {
			break
		}
		fmt.Print(v, ",")
	}
	fmt.Println()
	total := 0
	acc := make(chan int)
	go func() {
		for v := range [5]int{1, 2, 3, 4, 5} {
			total += v // only this goroutine touches total until it sends
		}
		acc <- total
	}()
	fmt.Println(<-acc, total)
}

// Output:
// 10,110,210,310,410,
// 10 10
