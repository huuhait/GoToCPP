// phase: 03

// An index out of range of a slice panics after the calls of its statement: f runs first.
package main

import "fmt"

var s = []int{1, 2, 3}

func f() int {
	fmt.Println("f ran")
	s = append(s, 4)
	return 1
}

func main() {
	i := 3
	fmt.Println(s[i], f())
	i = 4
	s[i] = f()
}
