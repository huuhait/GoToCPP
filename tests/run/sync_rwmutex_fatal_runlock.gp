// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: sync: RUnlock of unlocked RWMutex\n\ngoroutine 1 \[running\]:\n

// RUnlock of an RWMutex that is not read-locked is a fatal error.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var rw sync.RWMutex
	rw.RLock()
	rw.RUnlock()
	fmt.Println("one too many")
	rw.RUnlock()
}

// Output:
// one too many
