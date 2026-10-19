// phase: 08

// Writer preference: once a Lock is waiting, new readers wait behind it, so the writer gets the
// lock before a reader that arrived later.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var rw sync.RWMutex
	var mu sync.Mutex
	var order []string
	log := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}

	rw.RLock() // reader 1 holds the lock
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.Lock()
		log("writer")
		rw.Unlock()
	}()
	// The writer is pending when TryRLock starts to fail.
	for rw.TryRLock() {
		rw.RUnlock()
	}
	fmt.Println("writer is pending, TryRLock fails")
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.RLock()
		log("reader 2")
		rw.RUnlock()
	}()
	log("reader 1 leaves")
	rw.RUnlock()
	wg.Wait()
	fmt.Println(order)
}

// Output:
// writer is pending, TryRLock fails
// [reader 1 leaves writer reader 2]
