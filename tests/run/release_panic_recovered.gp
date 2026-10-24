// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: ^panic: first \[recovered\]\n\tpanic: again: first\n$

// A -release program prints a panic's chain as Go words it, and nothing else: a panic that a
// deferred call recovered and then panicked again with another value.
package main

import "fmt"

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r)
		panic(fmt.Sprint("again: ", r))
	}()
	panic("first")
}

// Output:
// recovered: first
