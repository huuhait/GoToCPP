// phase: 04

// <variant>: holds_alternative, get<I> / get<T> (a function template: the `[...]` is a
// template argument list), index(), and visit with a callable that takes the common type.
package main

import (
	"fmt"

	"<string>"
	"<variant>"
)

func main() {
	var v std.variant[int, std.string]
	fmt.Println(int(v.index()), std.holds_alternative[int](v))
	v = std.string("text")
	fmt.Println(int(v.index()), std.holds_alternative[std.string](v), string(std.get[std.string](v)))
	v = 12
	fmt.Println(int(std.get[0](v)), int(std.get[int](v)))


	// visit needs one callable for every alternative: a Go func literal is not generic, so it
	// takes the common type (C.double converts to int).
	var w std.variant[int, C.double]
	w = C.double(2.75)
	r := std.visit(func(x int) int { return x * 10 }, w)
	fmt.Println(int(r))
}

// Output:
// 0 true
// 1 true text
// 12 12
// 20
