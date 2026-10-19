// phase: 08

// Gosched with no other goroutine returns at once, any number of times.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	for i := 0; i < 1000; i++ {
		runtime.Gosched()
	}
	fmt.Println("done", runtime.NumGoroutine())
}

// Output:
// done 1
