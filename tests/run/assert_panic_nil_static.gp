// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: main\.J is nil, not main\.T

// "main.J is nil, not main.T": the static type of a nil operand (step 5.5).
package main

import "fmt"

type J interface{ N() }
type T struct{}

func (T) N() {}

func main() {
	var j J
	fmt.Println("start")
	t := j.(T)
	fmt.Println(t)
}

// Output:
// start
