// phase: 08

// A buffered channel as a counting semaphore: at most three goroutines are inside the critical
// section at once. A channel is the mutex that protects the counters.
package main

import "fmt"

func main() {
	const total, limit = 30, 3
	sem := make(chan struct{}, limit)
	mu := make(chan struct{}, 1)
	done := make(chan bool)
	inside, maxInside, entered := 0, 0, 0
	for i := 0; i < total; i++ {
		go func() {
			sem <- struct{}{} // acquire
			mu <- struct{}{}
			inside++
			entered++
			if inside > maxInside {
				maxInside = inside
			}
			<-mu
			// some work
			s := 0
			for k := 0; k < 1000; k++ {
				s += k
			}
			mu <- struct{}{}
			inside--
			<-mu
			<-sem // release
			done <- s > 0
		}()
	}
	ok := 0
	for i := 0; i < total; i++ {
		if <-done {
			ok++
		}
	}
	fmt.Println("entered", entered, "ok", ok, "never more than the limit:", maxInside <= limit, "left inside", inside)
}

// Output:
// entered 30 ok 30 never more than the limit: true left inside 0
