// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[sync\.Mutex\.Lock\]:\n

// A function passed to Once.Do that calls Do on the same Once waits for itself.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var once sync.Once
	fmt.Println("start")
	once.Do(func() {
		fmt.Println("in Do")
		once.Do(func() { fmt.Println("never") })
	})
	fmt.Println("never")
}

// Output:
// start
// in Do
