// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is string, not int

// A failed assertion to a concrete type panics with Go's message (step 5.5).
package main

import "fmt"

func main() {
	var x any = "text"
	fmt.Println("start")
	n := x.(int)
	fmt.Println(n)
}

// Output:
// start
