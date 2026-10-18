// phase: 07

// A generic function called with C++ values: go/types cannot infer the type arguments, so the instance
// is a C++ function template and the C++ compiler deduces them from the arguments (README §2.4).
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Id[T any](x T) T { return x }

func Pick[T any](first bool, a, b T) T {
	if first {
		return a
	}
	return b
}

func Twice[T any](x T) (T, T) { return x, x }

func Wrap[T any](x T) *T { return &x }

func Keep[T any](xs ...T) []T {
	var out []T
	for _, x := range xs {
		out = append(out, x)
	}
	return out
}

func main() {
	v := std.vector[int]{1, 2, 3}
	s := std.string("hello")
	a := Id(v)
	fmt.Println(int(a.size()), int(Id(s).size()))
	p := Pick(true, v, std.vector[int]{9})
	q := Pick(false, v, std.vector[int]{9})
	fmt.Println(int(p.size()), int(q.size()))
	x, y := Twice(s)
	fmt.Println(int(x.size()), int(y.size()))
	w := Wrap(v)
	fmt.Println(int(w.size()))
	var ks []std.string = Keep(s, s, std.string("z"))
	fmt.Println(len(ks), int(ks[2].size()))
}

// Output:
// 3 5
// 3 1
// 5 5
// 3
// 3 1
