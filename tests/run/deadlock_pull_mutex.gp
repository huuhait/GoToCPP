// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[coroutine\]:\n
// Stderr: \ngoroutine \d+ \[sync\.Mutex\.Lock\]:\n(?:.+\n\t.+\n)+created by iter\.Pull\[\.\.\.\] in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n$

// The sequence locks a Mutex that main holds, and main waits for the sequence.
package main

import (
	"fmt"
	"iter"
	"sync"
)

func main() {
	var mu sync.Mutex
	mu.Lock()
	seq := func(yield func(int) bool) {
		mu.Lock()
		yield(1)
		mu.Unlock()
	}
	next, stop := iter.Pull(seq)
	defer stop()
	fmt.Println("calling next")
	next()
	fmt.Println("not reached")
}

// Output:
// calling next
