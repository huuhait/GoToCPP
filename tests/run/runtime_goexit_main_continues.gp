// phase: 08

// A goroutine that calls Goexit ends alone: main goes on, and so do the other goroutines.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	results := make(chan string, 6)
	for i := 1; i <= 3; i++ {
		go func() {
			defer func() { results <- fmt.Sprint("goroutine ", i, " ended") }()
			if i == 2 {
				runtime.Goexit()
			}
			results <- fmt.Sprint("goroutine ", i, " ran to its end")
		}()
	}
	// Goroutines 1 and 3 send two messages, goroutine 2 one.
	count := map[string]int{}
	for range 5 {
		count[<-results]++
	}
	for i := 1; i <= 3; i++ {
		fmt.Println(i, count[fmt.Sprint("goroutine ", i, " ran to its end")], count[fmt.Sprint("goroutine ", i, " ended")])
	}
	fmt.Println("main is still here")
}

// Output:
// 1 1 1
// 2 0 1
// 3 1 1
// main is still here
