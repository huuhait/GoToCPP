// phase: 05
// ExitCode: 2
// Stderr: ^panic: first \[recovered\]\n\tpanic: second\n\ngoroutine 1 \[running\]:\nmain\.main\.func1\(\)\n\t/.+/recover_trace\.g[op]:15( \+0x[0-9a-f]+)?\npanic\([^\n]*\)\n\t[^\n]+\nmain\.main\(\)\n\t/.+/recover_trace\.g[op]:17( \+0x[0-9a-f]+)?\n$

// The trace of a panic in a deferred function that recovered the first one: the aware function's
// frame has its Go name, then the panic that it ran for.
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
