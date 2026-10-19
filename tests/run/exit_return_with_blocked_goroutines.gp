// phase: 08

// main returns while other goroutines are blocked on a channel: the program ends with code 0 and
// what main printed is all of the output, nothing waits for the goroutines and their deferred
// calls do not run.
package main

import "fmt"

func main() {
	never := make(chan int)
	started := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		go func() {
			defer fmt.Println("deferred call of a blocked goroutine: never printed")
			started <- struct{}{}
			<-never
		}()
	}
	for i := 0; i < 3; i++ {
		<-started
	}
	fmt.Println("main returns")
}

// Output:
// main returns
