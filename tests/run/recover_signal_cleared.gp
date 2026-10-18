// phase: 05
// ExitCode: 2
// Stderr: ^panic: later\n\ngoroutine 1 \[running\]:\n

// The recovery of a nil dereference clears its signal: the later panic prints no signal line.
package main

import "fmt"

type T struct{ n int }

func f() {
	defer func() { fmt.Println("recovered:", recover()) }()
	var p *T
	fmt.Println(p.n)
}

func main() {
	f()
	panic("later")
}

// Output:
// recovered: runtime error: invalid memory address or nil pointer dereference
