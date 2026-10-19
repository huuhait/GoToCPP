// phase: 08

// Stack cuts its output to the buffer: it returns the length of the buffer when the trace is
// longer, and 0 for an empty buffer.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	small := make([]byte, 10)
	n := runtime.Stack(small, false)
	fmt.Printf("%d %q\n", n, small[:n])
	fmt.Println(runtime.Stack(nil, false))
	big := make([]byte, 1<<16)
	m := runtime.Stack(big, false)
	fmt.Println(m > 10, m < len(big))
}

// Output:
// 10 "goroutine "
// 0
// true true
