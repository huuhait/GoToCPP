// phase: 03

// A slice expression out of range panics in its place among the calls, before a call after
// it runs; an index out of range panics after the calls of its statement.
package main

import "fmt"

var s = []int{1, 2, 3}

func f() int {
	fmt.Println("f ran")
	return 1
}

func main() {
	i := 1
	fmt.Println(s[i] + f())
	j := 4
	fmt.Println(len(s[:j]), f())
}
