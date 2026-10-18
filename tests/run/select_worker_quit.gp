// phase: 08

// A worker that does jobs until it is told to quit; the quit message is sent after all jobs were
// taken, so the result is the same on every run.
package main

import "fmt"

func worker(jobs <-chan int, quit <-chan struct{}, results chan<- int, stopped chan<- bool) {
	for {
		select {
		case j := <-jobs:
			results <- j * j
		case <-quit:
			stopped <- true
			return
		}
	}
}

func main() {
	jobs := make(chan int)
	quit := make(chan struct{})
	results := make(chan int, 10)
	stopped := make(chan bool)
	go worker(jobs, quit, results, stopped)
	for i := 1; i <= 5; i++ {
		jobs <- i // unbuffered: the worker has taken it when this returns
	}
	close(quit)
	fmt.Println(<-stopped)
	close(results)
	sum := 0
	for r := range results {
		sum += r
	}
	fmt.Println(sum)
}

// Output:
// true
// 55
