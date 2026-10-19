// phase: 08

// Goexit in a nested call runs the deferred calls of every frame, innermost first, and ends the
// goroutine.
package main

import (
	"fmt"
	"runtime"
)

func level3() {
	defer fmt.Println("level3 deferred")
	fmt.Println("level3 calls Goexit")
	runtime.Goexit()
	fmt.Println("level3 not reached")
}

func level2() {
	defer fmt.Println("level2 deferred")
	level3()
	fmt.Println("level2 not reached")
}

func level1() {
	defer fmt.Println("level1 deferred")
	level2()
	fmt.Println("level1 not reached")
}

func main() {
	done := make(chan bool)
	go func() {
		defer func() { done <- true }()
		level1()
	}()
	<-done
	fmt.Println("done")
}

// Output:
// level3 calls Goexit
// level3 deferred
// level2 deferred
// level1 deferred
// done
