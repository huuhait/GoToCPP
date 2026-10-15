// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[5\] with length 2\n\ngoroutine 1 \[running\]:\nmain\.get\([^\n]*\)\n\t/.+/init_panic_var\.g[op]:23( \+0x[0-9a-f]+)?\nmain\.init\(\)\n\t/.+/init_panic_var\.g[op]:19 \+0x[0-9a-f]+\n$

// A panic in a package variable's initializer (here in a function that it calls) stops the
// program before any init function and main run, with Go's trace: the initializers are in
// main.init, at the line of the variable. The variables before it in the initialization
// order are set.
package main

import "fmt"

var before = show("before")

var s = []int{1, 2}

var i = 5

var x = get(i)

func get(n int) int {
	fmt.Println("get", n)
	return s[n]
}

func show(name string) int {
	fmt.Println("set", name)
	return 1
}

func init() {
	fmt.Println("init")
}

func main() {
	fmt.Println("main", x)
}

// Output:
// set before
// get 5
