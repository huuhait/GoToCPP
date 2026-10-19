// phase: 08

// A chain of goroutines passes a value along unbuffered channels many times, each asleep most of
// the time; and a pipeline that ends by closing channels.
package main

import "fmt"

func main() {
	const n = 50
	first := make(chan int)
	in := first
	for i := 0; i < n; i++ {
		out := make(chan int)
		go func(in, out chan int) {
			for v := range in {
				out <- v + 1
			}
			close(out)
		}(in, out)
		in = out
	}
	go func() {
		for i := 0; i < 2000; i++ {
			first <- i
		}
		close(first)
	}()
	sum := 0
	for v := range in {
		sum += v
	}
	fmt.Println(sum)
}

// Output:
// 2099000
