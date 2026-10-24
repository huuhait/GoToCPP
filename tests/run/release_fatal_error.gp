// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: ^fatal error: sync: unlock of unlocked mutex\n$

// A -release program: a fatal error prints its line only, also while other goroutines wait (a
// debug build prints every goroutine then).
package main

import "sync"

func waiter(c chan int) {
	<-c
}

func main() {
	go waiter(make(chan int))
	var mu sync.Mutex
	mu.Unlock()
}

// Output:
