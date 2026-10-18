// phase: 07

// C++ types written as type arguments: std.vector[int] is spelled from the syntax, and a mix of Go and C++ type arguments works.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Zero[T any]() T {
	var z T
	return z
}

func Id[T any](x T) T { return x }

func Pair[A, B any](a A, b B) (B, A) { return b, a }

func Fill[T any](n int, v T) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func main() {
	z := Zero[std.vector[int]]()
	fmt.Println(int(z.size()))
	v := std.vector[int]{4, 5}
	b := Id[std.vector[int]](v)
	fmt.Println(int(b.size()))
	n, str := Pair[std.string, int](std.string("ab"), 7)
	fmt.Println(n, int(str.size()))
	var fs []std.string = Fill[std.string](3, std.string("xy"))
	fmt.Println(len(fs), int(fs[2].size()))
	fmt.Println(int(Zero[std.string]().size()))
}

// Output:
// 0
// 2
// 7 2
// 3 2
// 0
