// phase: 07

// A generic function over S ~[]T instantiated with a Go slice of a C++ type: C++ deduces S and T (the default of T is the element of S), and len, cap, index, slice, range and append work on it. A pointer method of a struct with a C++ field is called from a generic function.
package main

import (
	"fmt"

	"<vector>"
)

func First[S ~[]T, T any](s S) T { return s[0] }

func Len[S ~[]T, T any](s S) int { return len(s) }

type H struct {
	v std.vector[int]
	n int
}

func (h *H) Push(x int) { h.v.push_back(C.int(x)); h.n++ }

func Use[T any](h *H, x T) int {
	h.Push(1)
	return h.n
}

func main() {
	fmt.Println(First([]int{4, 5}), Len([]std.vector[int]{{1}, {2, 3}}))
	h := &H{}
	fmt.Println(Use(h, "a"), Use(h, 2))
}

func Idx[S ~[]T, T any](s S, i int) T { return s[i] }

func Sum[S ~[]T, T any](s S, f func(T) int) int {
	n := 0
	for _, v := range s {
		n += f(v)
	}
	return n
}

func Tail[S ~[]T, T any](s S) S { return s[1:] }

func Add[S ~[]T, T any](s S, v T) S { return append(s, v) }

func init() {
	v := []std.vector[int]{{1}, {2, 3}}
	fmt.Println(Idx(v, 1).size(), Sum(v, func(x std.vector[int]) int { return int(x.size()) }), len(Tail(v)), len(Add(v, std.vector[int]{})))
}

// Output:
// 2 3 1 3
// 4 2
// 1 2
