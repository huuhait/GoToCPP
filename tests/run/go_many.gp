// phase: 08

// Two thousand goroutines at the same time: each waits for the start signal (a closed channel), and
// then sends its number. The sum is the same however they interleave.
package main

import "fmt"

func main() {
	const n = 2000
	start := make(chan struct{})
	results := make(chan int)
	for i := 1; i <= n; i++ {
		go func() {
			<-start
			results <- i
		}()
	}
	close(start)
	sum := 0
	for i := 0; i < n; i++ {
		sum += <-results
	}
	fmt.Println(sum, n*(n+1)/2)
}

// Output:
// 2001000 2001000
