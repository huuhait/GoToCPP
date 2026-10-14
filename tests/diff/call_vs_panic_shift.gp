// phase: 02

// A negative shift count panics only after the calls of the same
// expression ran, also the call to the right of the shift (Go order). The
// first line indexes a string in range with a call as the index.
package main

import "fmt"

func tr(name string, v int) int {
	fmt.Println("called", name)
	return v
}

func main() {
	n := -1
	s := "abc"
	fmt.Println(int(s[tr("index", 1)]) + tr("next", 1))
	fmt.Println(tr("left", 1)<<n + tr("right", 2))
}
