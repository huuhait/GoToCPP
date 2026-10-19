// phase: 08

// A pipeline with a done channel: the consumer takes five values and closes done; every stage stops
// and no goroutine is left.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func gen(done <-chan struct{}, wg *sync.WaitGroup) <-chan int {
	out := make(chan int)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(out)
		for i := 1; ; i++ {
			select {
			case out <- i:
			case <-done:
				return
			}
		}
	}()
	return out
}

func square(done <-chan struct{}, in <-chan int, wg *sync.WaitGroup) <-chan int {
	out := make(chan int)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(out)
		for v := range in {
			select {
			case out <- v * v:
			case <-done:
				return
			}
		}
	}()
	return out
}

func main() {
	done := make(chan struct{})
	var wg sync.WaitGroup
	out := square(done, square(done, gen(done, &wg), &wg), &wg)
	for i := 0; i < 5; i++ {
		fmt.Println(<-out)
	}
	close(done)
	wg.Wait()
	for runtime.NumGoroutine() > 1 {
		time.Sleep(time.Millisecond)
	}
	fmt.Println("goroutines:", runtime.NumGoroutine())
}

// Output:
// 1
// 16
// 81
// 256
// 625
// goroutines: 1
