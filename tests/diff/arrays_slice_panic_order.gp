// phase: 03

// Slicing a nil pointer to an array panics in its place among the calls, before a call after
// it; indexing it panics after the calls.
package main

import "fmt"

var p *[3]int

func f() int {
	fmt.Println("f ran")
	return 1
}

func main() {
	fmt.Println(len(p), f())
	fmt.Println(len(p[:]), f())
}
