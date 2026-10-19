// A C++ value that no conversion takes to the Go type it goes to.
package main

import (
	"./shapes.hpp"
	"<cstdio>"
	"<cstdlib>"
)

func take(p *int) {}

func get() *int {
	return cstdlib.abs(1)
	panic("unreachable")
}

func main() {
	var q *int = cstdlib.abs(4); _ = q
	var n int = cstdio.stdout; _ = n
	var m int = shapes.make_counter(); _ = m
	var f float64 = shapes.make_point(); _ = f
	take(cstdlib.abs(2))
	_ = get
}
