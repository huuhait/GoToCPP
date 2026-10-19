// phase: 08

// TryLock and TryRLock on every combination of holders.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var rw sync.RWMutex
	fmt.Println("free:", rw.TryRLock(), rw.TryRLock())
	fmt.Println("with readers: TryLock", rw.TryLock())
	rw.RUnlock()
	rw.RUnlock()
	fmt.Println("writer:", rw.TryLock())
	fmt.Println("with a writer: TryLock", rw.TryLock(), "TryRLock", rw.TryRLock())
	rw.Unlock()
	fmt.Println("free again:", rw.TryLock())
	rw.Unlock()
	rw.RLock()
	rw.RLock()
	rw.RUnlock()
	rw.RUnlock()
	rw.Lock()
	rw.Unlock()
	fmt.Println("reusable:", rw.TryRLock())
	rw.RUnlock()
}

// Output:
// free: true true
// with readers: TryLock false
// writer: true
// with a writer: TryLock false TryRLock false
// free again: true
// reusable: true
