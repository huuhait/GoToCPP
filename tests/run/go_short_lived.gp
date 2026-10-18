// phase: 08

// Fifty thousand short goroutines, a few hundred at a time: the runtime reuses the threads of the
// finished ones. The output must not depend on that.
package main

import "fmt"

func square(x int, out chan<- int) { out <- x * x }

func main() {
	const total = 50000
	const window = 256
	out := make(chan int, window)
	sum, started, got := 0, 0, 0
	for started < total {
		if started-got >= window {
			sum += <-out
			got++
		}
		go square(started%100, out)
		started++
	}
	for got < total {
		sum += <-out
		got++
	}
	fmt.Println(sum)
}

// Output:
// 164175000
