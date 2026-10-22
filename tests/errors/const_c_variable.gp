// phase: 01

// A const value that holds a variable is not constant, even when the variable
// is a C/C++ one: go/types has no type for the value, so gocpp check says it.
package main

var global C.int = 4

func f(n C.int) C.int {
	const fromParam = n // ERROR "n \(C/C\+\+ value\) is not constant"
	const sum = global + 1 // ERROR "global \+ 1 \(C/C\+\+ value\) is not constant"
	return fromParam + sum
}

func main() {
	var x C.int = 2
	const d = x + 1 // ERROR "x \+ 1 \(C/C\+\+ value\) is not constant"
	const ok C.int = 3
	const fine = ok + 1
	println(d, fine, f(1))
}
