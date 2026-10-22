// phase: 05

// A variable that a returned closure captures lives in a heap box (gp::ptr) in the C++ (a captured
// variable that never outlives its function is a plain C++ variable: robust.gp). In the closure, gp locals shows
// the Go variable, judging by the line that declares it: n (int), not a *int, and p stays a *T.
// The session is boxed.gdb.
package main

import "fmt"

type T struct{ v int }

func counter(start int) func() int {
	n := 5
	p := &T{v: start}
	f := func() int {
		n++
		p.v++ // @inc
		return n + p.v
	}
	return f // @make
}

func main() {
	next := counter(5)
	fmt.Println(next(), next())
}
