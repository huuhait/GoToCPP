// phase: 08

// The output of goroutines that have finished before main returns is all there (the goroutines
// take turns, so the order is fixed).
package main

import "fmt"

func main() {
	const n = 20
	turn := make([]chan int, n+1)
	for i := range turn {
		turn[i] = make(chan int, 1)
	}
	for i := 0; i < n; i++ {
		go func() {
			sum := <-turn[i]
			fmt.Println("worker", i)
			turn[i+1] <- sum + i
		}()
	}
	turn[0] <- 0
	fmt.Println("sum", <-turn[n])
}

// Output:
// worker 0
// worker 1
// worker 2
// worker 3
// worker 4
// worker 5
// worker 6
// worker 7
// worker 8
// worker 9
// worker 10
// worker 11
// worker 12
// worker 13
// worker 14
// worker 15
// worker 16
// worker 17
// worker 18
// worker 19
// sum 190
