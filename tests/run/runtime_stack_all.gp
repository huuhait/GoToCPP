// phase: 08

// Stack with all = true starts with the calling goroutine's trace. (Go also lists the other
// goroutines; gocpp can't see the stacks of other threads, and writes the caller's only.)
package main

import (
	"fmt"
	"runtime"
)

func main() {
	hold := make(chan struct{})
	go func() { <-hold }()
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	fmt.Println(string(buf[:22]), n > 22)
	close(hold)
}

// Output:
// goroutine 1 [running]: true
