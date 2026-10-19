// phase: 09

// Workers that select on ctx.Done() and a job channel: every job is handled exactly once, then a
// cancel stops all of them and a late send is refused.
package main

import (
	"context"
	"fmt"
	"sync"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	jobs := make(chan int)
	results := make(chan int, 100)
	var wg sync.WaitGroup
	handled := make([]int, 4)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case j := <-jobs:
					handled[w]++
					results <- j * j
				}
			}
		}()
	}
	for i := 1; i <= 100; i++ {
		jobs <- i
	}
	cancel()
	wg.Wait()
	close(results)
	sum, n := 0, 0
	for r := range results {
		sum += r
		n++
	}
	total := 0
	for _, h := range handled {
		total += h
	}
	fmt.Println(n, sum, total)
	select {
	case jobs <- 1:
		fmt.Println("accepted")
	default:
		fmt.Println("no worker left")
	}
	fmt.Println(ctx.Err())
}

// Output:
// 100 338350 100
// no worker left
// context canceled
