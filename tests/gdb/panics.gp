// phase: 08

// Where a Go panic starts: gp catch panic stops at every one. A panic in a function without
// defers, and in a function that a deferred recover guards from below, is a C++ throw (catch throw
// stops there); the panic statement of a function with defers does not throw (ADR-085: it is
// handled in the function's own frame), so catch throw alone misses it. runtime.Goexit is a throw
// too, and a goroutine with defers that calls it stops once. The session is panics.gdb.
package main

import (
	"fmt"
	"runtime"
	"sync"
)

func direct() {
	panic("direct") // @direct
}

func withDefer() {
	defer func() { recover() }()
	panic("with defer") // @withdefer
}

func unrecovered() {
	defer fmt.Println("deferred")
	panic("unrecovered") // @unrecovered
}

func goexit() {
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		defer fmt.Println("exiting")
		runtime.Goexit() // @goexit
	}()
	done.Wait()
}

func main() {
	func() {
		defer func() { recover() }()
		direct()
	}()
	withDefer()
	goexit()
	fmt.Println("done")
	unrecovered()
}
