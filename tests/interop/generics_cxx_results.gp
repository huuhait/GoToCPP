// phase: 08

// The result of a generic call whose type arguments C++ deduces from a C++ value is a gocpp slice or
// map of C++ values, as the argument it comes from: len, cap, range, index, slice expressions, append
// and delete on it, directly and through a variable declared from it (r := slices.Insert(c, 1, cv),
// var r = ...), also in a generic function, Go values made of it (len(r) + 1, len(r) > 2), and a tuple
// result assigned to existing variables (v, ok = Get(m, k)).
package main

import (
	"fmt"
	"slices"

	"<string>"
)

func Grow[T any, S ~[]T](xs S, x T) S { return append(xs[:len(xs):len(xs)], x) }

func Get[M ~map[K]V, K comparable, V any](m M, k K) (V, bool) {
	v, ok := m[k]
	return v, ok
}

func Put[M ~map[K]V, K comparable, V any](m M, k K, v V) M {
	m[k] = v
	return m
}

func Count[T any](v T) int {
	c := []std.string{"a", "b"}
	r := slices.Insert(c, 1, std.string("q"))
	var r2 = slices.Insert(c, 0, std.string("z"))
	return len(r) + cap(r[:1]) + len(r2)
}

func main() {
	c := []std.string{"a", "b"}
	r := slices.Insert(c, 1, std.string("q"))
	fmt.Println(len(r), cap(r) >= 3)
	for i, s := range r {
		fmt.Println(i, s)
	}
	r = append(r, std.string("z"))
	fmt.Println(r[len(r)-1], r[1:2], len(r[1:]))
	t := slices.Insert(c, 0, std.string("w"))[1:]
	fmt.Println(len(t), t[0], len(slices.Insert(c, 0, std.string("v"))))
	fmt.Println(len(Grow(c, std.string("x"))), cap(Grow(c, std.string("y"))) >= 3)

	m := map[int]std.string{1: "a"}
	var v std.string
	var ok bool
	v, ok = Get(m, 1)
	fmt.Println(v, ok)
	_, ok = Get(m, 7)
	fmt.Println(ok)
	m2 := Put(m, 2, std.string("b"))
	fmt.Println(len(m2), m2[2])
	delete(m2, 1)
	fmt.Println(len(m), len(Put(m, 3, std.string("c"))))

	var w = slices.Insert(c, 0, std.string("w"))
	n := len(w) + 1
	if len(w) > 2 && cap(w[:1]) >= 3 {
		fmt.Println("long", n)
	}
	fmt.Println(Count(1), Count("s"))
}

// Output:
// 3 true
// 0 a
// 1 q
// 2 b
// z [q] 3
// 2 a 3
// 3 true
// a true
// false
// 2 b
// 1 2
// long 4
// 10 10
