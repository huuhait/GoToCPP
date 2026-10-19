// phase: 08

// sync and channels together: a WaitGroup closes the results channel, a mutex guards a map the
// workers fill, and a Once closes a done channel from whichever worker finishes first.
package main

import (
	"fmt"
	"sync"
)

func main() {
	jobs := make(chan int)
	results := make(chan int, 10)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				j, ok := <-jobs
				if !ok {
					return
				}
				results <- j * j
			}
		}()
	}
	go func() {
		for i := 1; i <= 100; i++ {
			jobs <- i
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()
	sum := 0
	for {
		r, ok := <-results
		if !ok {
			break
		}
		sum += r
	}
	fmt.Println("sum of squares:", sum)

	var mu sync.Mutex
	seen := map[int]string{}
	var once sync.Once
	done := make(chan struct{})
	var wg2 sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			mu.Lock()
			seen[i] = fmt.Sprint("worker ", i)
			mu.Unlock()
			once.Do(func() { close(done) })
		}()
	}
	<-done
	wg2.Wait()
	fmt.Println(len(seen), seen[7])
}

// Output:
// sum of squares: 338350
// 10 worker 7
