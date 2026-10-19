// phase: 08

// RLocker returns a Locker whose Lock is RLock; it works with sync.Cond and as a plain Locker.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var rw sync.RWMutex
	rl := rw.RLocker()
	rl.Lock()
	rl.Lock()
	fmt.Println("two readers, TryLock:", rw.TryLock())
	rl.Unlock()
	rl.Unlock()
	fmt.Println("no readers, TryLock:", rw.TryLock())
	rw.Unlock()

	// A condition variable whose lock is the read side.
	ready := false
	cond := sync.NewCond(rw.RLocker())
	var wg sync.WaitGroup
	got := make([]bool, 5)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cond.L.Lock()
			for !ready {
				cond.Wait()
			}
			got[i] = ready
			cond.L.Unlock()
		}()
	}
	rw.Lock()
	ready = true
	rw.Unlock()
	cond.Broadcast()
	wg.Wait()
	fmt.Println(got)

	var l sync.Locker = &rw
	l.Lock()
	fmt.Println("as Locker:", !rw.TryRLock())
	l.Unlock()
}

// Output:
// two readers, TryLock: false
// no readers, TryLock: true
// [true true true true true]
// as Locker: true
