// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[coroutine\]:\n
// Stderr: \ngoroutine \d+ \[chan receive\]:\n(?:.+\n\t.+\n)+created by iter\.Pull\[\.\.\.\] in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n$

// iter.Pull runs the sequence on a coroutine: main waits in next ("coroutine") while the sequence
// waits for a channel that nobody sends on. Go lists both, the coroutine as created by iter.Pull.
package main

import (
	"fmt"
	"iter"
)

func main() {
	c := make(chan int)
	seq := func(yield func(int) bool) {
		fmt.Println("the sequence waits")
		<-c
		yield(1)
	}
	next, stop := iter.Pull(seq)
	defer stop()
	next()
	fmt.Println("not reached")
}

// Output:
// the sequence waits
