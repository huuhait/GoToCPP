// phase: 07

// comparable and ==, copies and containers of C++ type arguments.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Eq[T comparable](a, b T) bool { return a == b }

func Count[T comparable](xs []T, v T) int {
	n := 0
	for _, x := range xs {
		if x == v {
			n++
		}
	}
	return n
}

func Dup[T any](x T) []T {
	out := []T{x}
	out = append(out, x)
	return out
}

func Last[T any](xs []T) T { return xs[len(xs)-1] }

func main() {
	a, b := std.string("a"), std.string("b")
	fmt.Println(Eq(a, a), Eq(a, b), Eq(std.vector[int]{1}, std.vector[int]{1}))
	var ds []std.string = Dup(a)
	fmt.Println(len(ds), Count(ds, a), Count(ds, b))
	fmt.Println(int(Last(ds).size()))
}

// Output:
// true false true
// 2 2 0
// 1
