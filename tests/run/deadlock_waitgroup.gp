// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[sync\.WaitGroup\.Wait\]:\n(?s:.*?)main\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// Wait on a WaitGroup whose counter never reaches zero.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { wg.Done() }()
	fmt.Println("waiting for two, one is coming")
	wg.Wait()
}

// Output:
// waiting for two, one is coming
