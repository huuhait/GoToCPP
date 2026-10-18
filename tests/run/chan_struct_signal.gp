// phase: 08

// chan struct{} as a signal: sends and receives of the empty struct, buffered counting, and close as
// a broadcast.
package main

import "fmt"

func main() {
	sem := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		sem <- struct{}{}
	}
	fmt.Println(len(sem), cap(sem))
	<-sem
	fmt.Println(len(sem))

	done := make(chan struct{})
	results := make(chan int, 5)
	for i := 0; i < 5; i++ {
		go func() {
			<-done
			results <- i
		}()
	}
	close(done) // wakes all five at once
	sum := 0
	for i := 0; i < 5; i++ {
		sum += <-results
	}
	fmt.Println(sum)
	_, ok := <-done
	fmt.Println(ok)
	var z struct{}
	t := make(chan struct{}, 1)
	t <- z
	fmt.Println(<-t == z)
}

// Output:
// 3 3
// 2
// 10
// false
// true
