// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[sync\.Cond\.Wait\]:\n(?s:.*?)main\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// Cond.Wait with nobody to Signal.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	c := sync.NewCond(&mu)
	mu.Lock()
	fmt.Println("waiting for a signal")
	c.Wait()
}

// Output:
// waiting for a signal
