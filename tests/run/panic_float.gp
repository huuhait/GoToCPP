// phase: 02
// ExitCode: 2
// Stderr: ^panic: \+1\.500000e\+000\n

// panic with a float prints it in the runtime's format, like println.
package main

import "fmt"

func main() {
	fmt.Println("half way")
	panic(1.5)
}

// Output:
// half way
