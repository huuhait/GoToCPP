// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface is nil, not main\.Shape

// An assertion to an interface type on a nil interface: "interface conversion: interface is nil,
// not main.Shape" (step 5.5).
package main

import "fmt"

type Shape interface{ Area() float64 }

func main() {
	var x any
	fmt.Println("start")
	s := x.(Shape)
	fmt.Println(s.Area())
}

// Output:
// start
