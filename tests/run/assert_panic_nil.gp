// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is nil, not int

// An assertion on a nil interface panics with "interface conversion: interface {} is nil, not
// int" (step 5.5).
package main

import "fmt"

func main() {
	var x any
	fmt.Println("start")
	fmt.Println(x.(int))
}

// Output:
// start
