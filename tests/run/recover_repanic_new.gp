// phase: 05
// ExitCode: 2
// Stderr: ^panic: first \[recovered\]\n\tpanic: second\n\ngoroutine 1 \[running\]:\n

// A new panic after a recover: the recovered one stays in the chain, marked [recovered].
package main

import "fmt"

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered", r)
		panic("second")
	}()
	panic("first")
}

// Output:
// recovered first
