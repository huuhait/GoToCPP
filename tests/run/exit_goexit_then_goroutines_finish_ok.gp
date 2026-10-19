// phase: 08

// A goroutine that ends by Goexit while main returns normally: exit code 0.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer fmt.Println("deferred in the goroutine")
		runtime.Goexit()
	}()
	<-done
	fmt.Println("main returns")
}

// Output:
// deferred in the goroutine
// main returns
