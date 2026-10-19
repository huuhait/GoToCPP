// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: sync: unlock of unlocked mutex\n\ngoroutine 1 \[running\]:\n

// Unlocking an unlocked mutex is a fatal error, not a panic: exit code 2, and the deferred call
// does not run.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	defer fmt.Println("deferred calls do not run")
	fmt.Println("before")
	mu.Unlock()
	fmt.Println("after")
}

// Output:
// before
