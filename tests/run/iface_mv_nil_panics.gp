// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// Evaluating the method value of a nil interface panics THERE (step 5.9), not where the value is
// called: "made" is never printed.
package main

import "fmt"

type Greeter interface{ Greet() string }

func main() {
	var g Greeter
	fmt.Println("making")
	f := g.Greet
	fmt.Println("made")
	fmt.Println(f())
}

// Output:
// making
