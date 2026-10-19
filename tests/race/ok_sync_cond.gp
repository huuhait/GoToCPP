// phase: 08

// A Cond wakes a waiter that reads what the signaler wrote under the lock.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	ready := false
	data := 0
	done := make(chan int)
	go func() {
		mu.Lock()
		for !ready {
			cond.Wait()
		}
		v := data
		mu.Unlock()
		done <- v
	}()
	mu.Lock()
	data = 42
	ready = true
	mu.Unlock()
	cond.Signal()
	fmt.Println(<-done)
}

// Output:
// 42
