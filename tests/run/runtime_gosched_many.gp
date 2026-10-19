// phase: 08

// Goroutines that call Gosched in a loop all finish, and the sum of what they computed does not
// depend on the order. (Go's Gosched with GOMAXPROCS(1) would give an order: gocpp's goroutines
// are threads, so a test can't rely on one.)
package main

import (
	"fmt"
	"runtime"
)

func main() {
	const workers = 8
	results := make(chan int, workers)
	for w := 0; w < workers; w++ {
		go func() {
			sum := 0
			for i := 1; i <= 100; i++ {
				sum += i * w
				runtime.Gosched()
			}
			results <- sum
		}()
	}
	total := 0
	for w := 0; w < workers; w++ {
		total += <-results
	}
	fmt.Println(total)
}

// Output:
// 141400
