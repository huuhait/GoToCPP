// phase: 08

// Readers share an RWMutex: all of them hold the read lock at the same time. (If RLock were
// exclusive the program would never get past the barrier.)
package main

import (
	"fmt"
	"sync"
)

func main() {
	var rw sync.RWMutex
	const readers = 8
	var inside sync.WaitGroup
	inside.Add(readers)
	allIn := make(chan struct{})
	go func() {
		inside.Wait()
		close(allIn)
	}()
	var done sync.WaitGroup
	for i := 0; i < readers; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			rw.RLock()
			inside.Done()
			<-allIn // every reader holds the lock now
			rw.RUnlock()
		}()
	}
	done.Wait()
	fmt.Println("all", readers, "readers held the lock together")
	fmt.Println("writer can lock now:", rw.TryLock())
	rw.Unlock()
}

// Output:
// all 8 readers held the lock together
// writer can lock now: true
