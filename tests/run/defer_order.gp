// phase: 03
// Stderr: ^println 2 1 11\n$

// defer runs the deferred calls when the function returns, the last one first; defers in a
// loop accumulate until then; the function and its arguments are evaluated at the defer
// statement, in Go's order (gc evaluates the arguments of defer f(x, g()) like those of the
// call f(x, g()): g runs first, then x is read), so later changes don't reach them.
package main

import "fmt"

var g = 10

func bump() int {
	g++
	return g
}

func inc(p *int) int {
	*p++
	return *p
}

func show(tag string, a, b int) { fmt.Println(tag, a, b) }

func trace(s string) string {
	fmt.Println("enter", s)
	return s
}

func leave(s string) { fmt.Println("leave", s) }

func nested() {
	defer leave(trace("nested"))
	fmt.Println("in nested")
}

func main() {
	defer fmt.Println("main: last line")
	for i := range 3 {
		defer fmt.Println("loop", i)
	}
	x := 1
	defer show("x", x, inc(&x))
	defer show("g", g, bump())
	s := []int{10, 20, 30}
	i := 0
	defer show("s", s[i], inc(&i))
	defer println("println", x, i, g)
	x, i, g = 100, 2, 1000
	nested()
	fmt.Println("body done", x, i, g)
}

// Output:
// enter nested
// in nested
// leave nested
// body done 100 2 1000
// s 20 1
// g 11 11
// x 2 2
// loop 2
// loop 1
// loop 0
// main: last line
