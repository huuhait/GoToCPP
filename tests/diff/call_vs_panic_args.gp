// phase: 02

// A division by zero in one argument panics only after the calls in the
// other arguments ran, also those to its right (Go order).
package main

import "fmt"

func tr(name string, v int) int {
	fmt.Println("called", name)
	return v
}

func main() {
	zero := 0
	fmt.Println(7/1, tr("ok", 1))
	fmt.Println(7/zero, tr("arg", 1))
}
