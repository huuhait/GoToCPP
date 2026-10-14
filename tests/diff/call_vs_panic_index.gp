// phase: 02

// A string index out of range panics only after the calls of the same
// expression ran, also the call to the right of the index (Go order).
package main

import "fmt"

func tr(name string, v int) int {
	fmt.Println("called", name)
	return v
}

func main() {
	s := "abc"
	i := 5
	fmt.Println(int(s[1]) + tr("first", 1))
	fmt.Println(int(s[i]) + tr("next", 1))
}
