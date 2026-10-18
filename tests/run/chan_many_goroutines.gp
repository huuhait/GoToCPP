// phase: 08

// Ten thousand goroutines blocked on one channel at the same time; closing it releases them all.
package main

import "fmt"

func main() {
	const n = 10000
	gate := make(chan struct{})
	ready := make(chan bool, n)
	results := make(chan int, n)
	for i := 0; i < n; i++ {
		go func() {
			ready <- true
			<-gate
			results <- i % 10
		}()
	}
	for i := 0; i < n; i++ {
		<-ready
	}
	close(gate)
	sum := 0
	for i := 0; i < n; i++ {
		sum += <-results
	}
	fmt.Println(sum)
}

// Output:
// 45000
