// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: sync: unlock of unlocked mutex\n\ngoroutine [0-9]+ \[running\]:\n

// The fatal error of a goroutine ends the whole program, with exit code 2.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		defer close(done)
		mu.Unlock()
	}()
	<-done
	fmt.Println("not reached")
}

// Output:
