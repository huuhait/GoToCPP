// phase: 08

// The main goroutine is the only goroutine of a new program, and a goroutine that was started and
// has ended is not counted any more.
package main

import (
	"fmt"
	"runtime"
)

func waitFor(n int) {
	for runtime.NumGoroutine() != n {
		runtime.Gosched()
	}
}

func main() {
	fmt.Println(runtime.NumGoroutine())
	done := make(chan struct{})
	go func() { close(done) }()
	<-done
	waitFor(1)
	fmt.Println(runtime.NumGoroutine())
}

// Output:
// 1
// 1
