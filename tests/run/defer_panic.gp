// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[5\] with length 3\n\ngoroutine 1 \[running\]:\nmain\.at\([^\n]*\)\n\t/.+/defer_panic\.g[op]:16( \+0x[0-9a-f]+)?\nmain\.inner\(\)\n\t/.+/defer_panic\.g[op]:21 \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/defer_panic\.g[op]:29 \+0x[0-9a-f]+\n$

// A panic runs the deferred calls of every function that it leaves, the innermost first, and
// is then reported as it would be without them: the same message, and the trace of the place
// where it started, not of the deferred calls.
package main

import "fmt"

func cleanup(name string) { fmt.Println("cleanup", name) }

func at(s []int, i int) int {
	defer cleanup("at")
	return s[i]
}

func inner() {
	defer cleanup("inner")
	fmt.Println(at([]int{1, 2, 3}, 5))
	fmt.Println("not reached")
}

func main() {
	defer cleanup("main")
	defer fmt.Println("main's second defer")
	fmt.Println("start")
	inner()
}

// Output:
// start
// cleanup at
// cleanup inner
// main's second defer
// cleanup main
