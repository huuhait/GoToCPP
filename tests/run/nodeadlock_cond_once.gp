// phase: 08

// Cond.Wait / Broadcast, Once and a producer/consumer queue: every wait has a waker that is not
// asleep.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	queue := []int{}
	var wg sync.WaitGroup
	sum := 0
	for c := 0; c < 4; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				for len(queue) == 0 {
					cond.Wait()
				}
				v := queue[0]
				queue = queue[1:]
				mu.Unlock()
				if v < 0 {
					return
				}
				mu.Lock()
				sum += v
				mu.Unlock()
			}
		}()
	}
	for i := 1; i <= 1000; i++ {
		mu.Lock()
		queue = append(queue, i)
		mu.Unlock()
		cond.Signal()
	}
	for c := 0; c < 4; c++ {
		mu.Lock()
		queue = append(queue, -1)
		mu.Unlock()
		cond.Broadcast()
	}
	wg.Wait()
	fmt.Println(sum)
	var once sync.Once
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); once.Do(func() { fmt.Println("once") }) }()
	}
	wg.Wait()
}

// Output:
// 500500
// once
