// Package variables: constants at compile time (constinit), the others in Go's
// initialization order, then the init functions in source order.
package main

import "fmt"

var a = b + 1 // after b
var b = next("b")
var c, d = pair()
var _ = next("blank")

var (
	n         = 42
	big       = 1 << 62
	s         = "str"
	empty     = ""
	f         = 0.5
	t         = true
	zero      int
	zeroS     string
	counter   = 0
	initCount int
)

const k = 7

func next(name string) int {
	counter = counter + 1
	println("init", name, counter)
	return counter * 10
}

func pair() (int, string) {
	return next("pair"), "d"
}

func init() {
	initCount = initCount + 1
	println("first init", a, b)
}

func init() {
	initCount = initCount + 1
	println("second init", initCount)
}

func main() {
	fmt.Println(a, b, c, d, n, big, s, empty, f, t, zero, zeroS, k, initCount)
	n = n + k
	fmt.Println(n)
}
