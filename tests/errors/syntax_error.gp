// phase: 01

package main

// A package with a syntax error is not type-checked, like in go build:
// the type error in g is not reported.

func main() {
	x := 1 2 // ERROR "expected ';', found 2"
	_ = x
}

func g() {
	var s string = 1
}
