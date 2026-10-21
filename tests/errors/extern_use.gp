// phase: 04

// An extern function (//gocpp:extern, ADR-128) is only called; a function without a body needs
// the directive.

package main

//gocpp:extern C
func add3(a, b, c C.int) C.int

func noBody(x int) int // ERROR "missing function body \(to declare a function that C or C\+\+ defines, put //gocpp:extern in its doc comment\)"

func use() {
	_ = add3(1, 2, 3)
	f := add3 // ERROR "cannot use extern function add3 as a value: it is a C or C\+\+ function, only a call is possible"
	_ = f
	var g func(C.int, C.int, C.int) C.int
	g = add3 // ERROR "cannot use extern function add3 as a value"
	_ = g
	defer add3(1, 2, 3)
}

func main() { use() }
