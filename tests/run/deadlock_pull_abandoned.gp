// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\n
// Stderr: \ngoroutine \d+ \[coroutine\]:\n(?:.+\n\t.+\n)+created by iter\.Pull\[\.\.\.\] in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n$

// A Pull that is abandoned (stop is never called) leaves its coroutine waiting in yield; main
// then waits for a channel for ever: both are listed.
package main

import (
	"fmt"
	"iter"
)

func main() {
	seq := func(yield func(int) bool) {
		for i := 1; ; i++ {
			if !yield(i) {
				return
			}
		}
	}
	next, _ := iter.Pull(seq)
	v, ok := next()
	fmt.Println(v, ok)
	<-make(chan int)
}

// Output:
// 1 true
