// phase: 03

// A write through a nil pointer panics (exit code 2) after the calls of its statement.
package main

import "fmt"

func value() int {
	fmt.Println("value")
	return 1
}

func main() {
	var p *int
	fmt.Println(p == nil)
	*p = value()
	fmt.Println("not reached")
}
