// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: go of nil func value\n

// go f() with a nil func value is a fatal error at the go statement, in the calling goroutine.
package main

import "fmt"

func main() {
	fmt.Println("before")
	var f func()
	go f()
	fmt.Println("not reached")
}

// Output:
// before
