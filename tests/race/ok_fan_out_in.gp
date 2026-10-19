// phase: 08

// Each goroutine writes its own element of a slice, and main reads them after every done signal.
package main

import "fmt"

func main() {
	results := make([]int, 8)
	done := make(chan bool)
	for i := range results {
		go func() {
			results[i] = i * i
			done <- true
		}()
	}
	for range results {
		<-done
	}
	fmt.Println(results)
}

// Output:
// [0 1 4 9 16 25 36 49]
