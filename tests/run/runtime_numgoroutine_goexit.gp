// phase: 08

// A goroutine that ended by Goexit, and one that ended by a recovered panic, are not counted.
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
	a := make(chan struct{})
	b := make(chan struct{})
	go func() {
		defer close(a)
		runtime.Goexit()
	}()
	go func() {
		defer close(b)
		defer func() { recover() }()
		panic("recovered")
	}()
	<-a
	<-b
	waitFor(1)
	fmt.Println(runtime.NumGoroutine())
}

// Output:
// 1
