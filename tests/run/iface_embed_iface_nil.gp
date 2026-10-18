// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A method promoted from a nil embedded interface panics with Go's nil dereference message,
// also when it is reached through another interface.
package main

import "fmt"

type Speaker interface{ Speak() string }

type Named struct {
	Speaker
	tag string
}

func call(s Speaker) string { return s.Speak() }

func main() {
	n := Named{tag: "t"}
	fmt.Println("start", n.tag, n.Speaker == nil)
	fmt.Println(call(n))
}

// Output:
// start t true
