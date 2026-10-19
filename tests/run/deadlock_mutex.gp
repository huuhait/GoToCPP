// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[sync\.Mutex\.Lock\]:\n(?s:.*?)main\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A goroutine that locks a Mutex it holds already waits for ever (Go's reason "sync.Mutex.Lock").
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	mu.Lock()
	fmt.Println("locked once")
	mu.Lock()
	fmt.Println("not reached")
}

// Output:
// locked once
