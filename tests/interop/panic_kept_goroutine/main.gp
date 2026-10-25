// phase: 08
// ExitCode: 2
// Stderr: ^panic: in goroutine\n\ngoroutine 1 \[running\]:\n

// A Go panic of a goroutine that C++ code kept (std::current_exception, xp.hpp) and swallowed,
// thrown again in main: each time through functions with deferred calls that don't recover it,
// recovered, then once in a deferred call that recovers it and panics again (recovered outside),
// and at last unrecovered: the program ends with it, and the report doesn't call it recovered.
package main

import (
	"fmt"
	"sync"

	"./xp.hpp"
)

func passThrough(v string) {
	defer func() { fmt.Println("deferred for", v) }()
	func() {
		defer func() {}()
		xp.call_save(func() C.int {
			panic(v)
		})
	}()
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		xp.swallow(func() C.int {
			passThrough("in goroutine")
			return 0
		})
	}()
	wg.Wait()
	for i := 0; i < 2; i++ {
		func() {
			defer func() { fmt.Println("again", i, recover()) }()
			func() {
				defer func() { fmt.Println("through a defer") }()
				xp.rethrow_saved()
			}()
		}()
	}
	// a repanic of the kept panic in a deferred call, recovered
	func() {
		defer func() { fmt.Println("outer:", recover()) }()
		defer func() {
			x := recover()
			panic(fmt.Sprint("re:", x))
		}()
		xp.rethrow_saved()
	}()
	// finally uncaught: the program ends with the kept panic
	xp.rethrow_saved()
}

// Output:
// deferred for in goroutine
// swallowed
// through a defer
// again 0 in goroutine
// through a defer
// again 1 in goroutine
// outer: re:in goroutine
