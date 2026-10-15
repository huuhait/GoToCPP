// phase: 03
// ExitCode: 2
// Stderr: ^panic: init failed\n\ngoroutine 1 \[running\]:\nmain\.check\([^\n]*\)\n\t/.+/init_panic_func\.g[op]:16( \+0x[0-9a-f]+)?\nmain\.init\.1\(\)\n\t/.+/init_panic_func\.g[op]:27 \+0x[0-9a-f]+\n$

// A panic in the second init function: the first one ran to its end, the second one's
// deferred calls run, the third init function and main don't. Go names init functions
// main.init.0, main.init.1, ... in its traces, in source order.
package main

import "fmt"

var ready = false

func check(ok bool) {
	if !ok {
		panic("init failed")
	}
}

func init() {
	fmt.Println("init 0")
}

func init() {
	defer fmt.Println("init 1 deferred")
	fmt.Println("init 1")
	check(ready)
}

func init() {
	fmt.Println("init 2")
}

func main() {
	fmt.Println("main")
}

// Output:
// init 0
// init 1
// init 1 deferred
