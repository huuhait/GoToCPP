// phase: 08

// A three-stage pipeline: generate, square, filter. Each stage is a goroutine, connected by channels.
package main

import "fmt"

func gen(n int) <-chan int {
	out := make(chan int)
	go func() {
		for i := 1; i <= n; i++ {
			out <- i
		}
		close(out)
	}()
	return out
}

func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		for {
			v, ok := <-in
			if !ok {
				break
			}
			out <- v * v
		}
		close(out)
	}()
	return out
}

func keepOdd(in <-chan int) <-chan int {
	out := make(chan int, 4)
	go func() {
		for {
			v, ok := <-in
			if !ok {
				break
			}
			if v%2 == 1 {
				out <- v
			}
		}
		close(out)
	}()
	return out
}

func main() {
	c := keepOdd(square(gen(12)))
	for {
		v, ok := <-c
		if !ok {
			break
		}
		fmt.Print(v, ",")
	}
	fmt.Println()
}

// Output:
// 1,9,25,49,81,121,
