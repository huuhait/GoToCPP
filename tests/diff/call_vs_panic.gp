// phase: 02

// A call to the left of a division that panics runs before the panic, and
// so does a call to its right (Go evaluates the calls of an expression
// before its operators): the output so far and the exit code 2 must match.
package main

import "fmt"

func tr(name string, v int) int {
	fmt.Println("called", name)
	return v
}

func main() {
	zero := 0
	a := 10
	fmt.Println(tr("ok", 1) + a/2)
	x := a/zero + tr("right of the division", 1)
	fmt.Println("not reached", x)
}
