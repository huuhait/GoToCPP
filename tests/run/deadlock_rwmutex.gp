// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[sync\.RWMutex\.RLock\]:\n(?s:.*?)main\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A read lock while the write lock is held by the same goroutine.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.RWMutex
	mu.Lock()
	fmt.Println("write locked")
	mu.RLock()
}

// Output:
// write locked
