// phase: 08

// Goroutines blocked on a channel are counted until they end.
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
	const n = 25
	release := make(chan struct{})
	started := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			started <- struct{}{}
			<-release
		}()
	}
	for i := 0; i < n; i++ {
		<-started
	}
	fmt.Println("blocked:", runtime.NumGoroutine())
	close(release)
	waitFor(1)
	fmt.Println("after release:", runtime.NumGoroutine())
}

// Output:
// blocked: 26
// after release: 1
