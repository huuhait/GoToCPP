// phase: 08
// ExitCode: 2
// Stderr: (?s)^fatal error: sync: unlock of unlocked mutex\n\ngoroutine \d+ \[running\]:\n(?:.+\n\t.+\n)+created by main\.main in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A fatal error in a goroutine: its trace also ends with "created by".
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	fmt.Println("starting")
	go func() {
		mu.Unlock()
	}()
	<-make(chan int)
}

// Output:
// starting
