// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: sync: unlock of unlocked mutex\n

// recover() can't stop the fatal error of an unlock of an unlocked mutex.
package main

import (
	"fmt"
	"sync"
)

func main() {
	defer func() {
		fmt.Println("recovered:", recover())
	}()
	var mu sync.Mutex
	mu.Lock()
	mu.Unlock()
	fmt.Println("unlocking again")
	mu.Unlock()
}

// Output:
// unlocking again
