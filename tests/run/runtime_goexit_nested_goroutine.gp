// phase: 08

// Goexit only ends the calling goroutine: the goroutine that started it and the ones it started go on.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	child := make(chan string)
	parent := make(chan string)
	go func() {
		go func() {
			child <- "child outlives the goroutine that started it"
		}()
		defer func() { parent <- "parent deferred ran" }()
		runtime.Goexit()
	}()
	fmt.Println(<-parent)
	fmt.Println(<-child)
}

// Output:
// parent deferred ran
// child outlives the goroutine that started it
