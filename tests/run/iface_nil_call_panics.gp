// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference

// A method of a nil interface panics like a nil pointer dereference (step 5.4).
package main

import "fmt"

type Greeter interface{ Greet() string }

func main() {
	var g Greeter
	fmt.Println("before")
	fmt.Println(g.Greet())
	fmt.Println("after")
}

// Output:
// before
