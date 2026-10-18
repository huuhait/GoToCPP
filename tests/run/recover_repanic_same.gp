// phase: 05
// ExitCode: 2
// Stderr: ^panic: boom \[recovered, repanicked\]\n\ngoroutine 1 \[running\]:\n

// Panicking again with the recovered value: Go prints the first panic once, with
// "[recovered, repanicked]".
package main

import "fmt"

func main() {
	defer fmt.Println("main's defer runs")
	defer func() {
		r := recover()
		fmt.Println("recovered, panicking again:", r)
		panic(r)
	}()
	panic("boom")
}

// Output:
// recovered, panicking again: boom
// main's defer runs
