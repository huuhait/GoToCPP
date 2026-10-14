// phase: 03
// ExitCode: 2
// Stderr: ^panic: first\n\tpanic: second\n\tpanic: third\n\ngoroutine 1 \[running\]:\n
// Stderr: \ngoroutine 1 \[running\]:\nmain\.boom\([^\n]*\)\n\t/.+/defer_panic_chain\.g[op]:15( \+0x[0-9a-f]+)?\npanic\([^\n]*\)\n\t[^\n]+\nmain\.boom\([^\n]*\)\n\t/.+/defer_panic_chain\.g[op]:15( \+0x[0-9a-f]+)?\npanic\([^\n]*\)\n\t[^\n]+\nmain\.work\(\)\n\t/.+/defer_panic_chain\.g[op]:24 \+0x[0-9a-f]+\nmain\.main\(\)\n\t/.+/defer_panic_chain\.g[op]:29 \+0x[0-9a-f]+\n$

// A deferred call that panics while a panic is running doesn't stop the other deferred calls:
// they still run, and the new panic goes on. Go prints the whole chain, the first panic first,
// and the trace shows each deferred call on top of the panic that ran it ("panic(...)" frames;
// gocpp has no line for that frame, Go shows its runtime's panic.go).
package main

import "fmt"

func boom(s string) {
	panic(s)
}

func cleanup() { fmt.Println("cleanup") }

func work() {
	defer cleanup()
	defer boom("third")
	defer boom("second")
	panic("first")
}

func main() {
	defer fmt.Println("main's defer runs too")
	work()
}

// Output:
// cleanup
// main's defer runs too
