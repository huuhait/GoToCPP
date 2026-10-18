// phase: 08

// A variable that goroutines and main share is boxed: they all see the one variable. The channel
// orders the accesses, so the result is exact.
package main

import "fmt"

func main() {
	counter := 0
	lock := make(chan struct{}, 1)
	done := make(chan bool)
	const workers, each = 8, 500
	for w := 0; w < workers; w++ {
		go func() {
			for i := 0; i < each; i++ {
				lock <- struct{}{}
				counter++
				<-lock
			}
			done <- true
		}()
	}
	for w := 0; w < workers; w++ {
		<-done
	}
	fmt.Println(counter, workers*each)

	var results [4]int
	for i := range results {
		go func() {
			results[i] = i * i // each goroutine writes its own element
			done <- true
		}()
	}
	for range results {
		<-done
	}
	fmt.Println(results)
}

// Output:
// 4000 4000
// [0 1 4 9]
