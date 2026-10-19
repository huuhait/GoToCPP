// phase: 08

// Readers hold an RWMutex at the same time (a barrier inside the read lock only opens when all of
// them are in), while a writer excludes them.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var rw sync.RWMutex
	const readers = 8
	var in sync.WaitGroup
	in.Add(readers)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rw.RLock()
			in.Done()
			in.Wait() // everybody holds the read lock now
			rw.RUnlock()
		}()
	}
	wg.Wait()
	fmt.Println("all readers overlapped")
	rw.Lock()
	fmt.Println("writer got it:", rw.TryRLock(), rw.TryLock())
	rw.Unlock()
	fmt.Println(rw.TryRLock(), rw.TryRLock(), rw.TryLock())
	rw.RUnlock()
	rw.RUnlock()
	fmt.Println(rw.TryLock())
}

// Output:
// all readers overlapped
// writer got it: false false
// true true false
// true
