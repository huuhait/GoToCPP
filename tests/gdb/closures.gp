// phase: 05

// Func literals that never escape stay plain C++ lambdas with a type that has no name (ADR-114).
// gp locals shows the Go type from the literal on the line that declares the variable. An
// unnamed type is a closure only in a frame of a .gp file, and a Go anonymous struct (named
// gp_anon_ in the C++) is never one. (A line with two names and two literals is a pair of
// gp::func in the C++: the signature of such a line is the unit test's, test_gocpp_gdb.py.) A
// variable that a local closure captures by reference prints as its value, as the variable itself
// does in its function: print k in show is 3, not (int64_t &) @0x...: 3.
// The session is closures.gdb.
package main

import "fmt"

func main() {
	k := 3
	add := func(x int) int { return x + k }
	greet := func(who string, n int) (string, error) { return who + "!", nil }
	var anon struct{ x int }
	anon.x = k
	text, err := greet("hi", 2)
	fmt.Println(add(1), text, err, anon) // @show
	show := func() {
		fmt.Println(k) // @ref
	}
	show()
}
