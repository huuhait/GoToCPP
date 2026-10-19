// phase: 08

// A goroutine that starts another goroutine and ends first: the count follows each of them.
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
	hold := make(chan struct{})
	inner := make(chan struct{})
	go func() {
		go func() {
			close(inner)
			<-hold
		}()
	}()
	<-inner
	waitFor(2)
	fmt.Println("the outer goroutine has ended:", runtime.NumGoroutine())
	close(hold)
	waitFor(1)
	fmt.Println("both have ended:", runtime.NumGoroutine())
}

// Output:
// the outer goroutine has ended: 2
// both have ended: 1
