// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: sync: Unlock of unlocked RWMutex\n\ngoroutine 1 \[running\]:\n

// Unlock of an RWMutex that is not write-locked is a fatal error.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var rw sync.RWMutex
	fmt.Println("before")
	rw.Unlock()
}

// Output:
// before
