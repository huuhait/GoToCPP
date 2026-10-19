// phase: 08

// The deferred calls of main run before the program ends, last first, and their output is flushed.
package main

import "fmt"

func main() {
	defer fmt.Println("first deferred, runs last")
	defer fmt.Println("second deferred, runs first")
	fmt.Println("main's body")
}

// Output:
// main's body
// second deferred, runs first
// first deferred, runs last
