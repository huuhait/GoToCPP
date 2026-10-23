// phase: 07

// A type parameter with a core slice or map type and a C++ part: S ~[]T written as []std.string (F[[]std.string]),
// M ~map[K]V written as map[int]std.string or inferred from a map[int]std.string, also when go/types records no
// instance (an argument that is a C++ value), and a written slice or map of a Go type with C++ parts ([]Holder). The
// instance's S is []T and M is map[K]V, so the body ranges over, indexes, appends to, reads with comma-ok, assigns and
// deletes in them; the written form and the inferred one agree.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Count[S ~[]T, T any](xs S) int {
	n := 0
	for _, x := range xs {
		_ = x
		n++
	}
	return n
}

func Last[S ~[]T, T any](xs S) T {
	var z T
	for _, x := range xs {
		z = x
	}
	if len(xs) > 0 {
		z = xs[len(xs)-1]
	}
	return z
}

func Grow[T any, S ~[]T](xs S, x T) S {
	return append(xs[:len(xs):len(xs)], x)
}

func Keys[M ~map[K]V, K comparable, V any](m M) int {
	n := 0
	for k, v := range m {
		_, _ = k, v
		n++
	}
	return n
}

func Get[M ~map[K]V, K comparable, V any](m M, k K) (V, bool) {
	v, ok := m[k]
	return v, ok
}

func Put[M ~map[K]V, K comparable, V any](m M, k K, v V) M {
	m[k] = v
	delete(m, k)
	m[k] = v
	return m
}

func ByName[M ~map[string]V, V any](m M) int { return len(m) }

type Holder struct {
	s std.string
	n int
}

func Vals[M ~map[K]V, K comparable, V any](m M) int {
	n := 0
	for _, v := range m {
		_ = v
		n++
	}
	return n
}

func main() {
	words := []std.string{"a", "bcd"}
	fmt.Println(Count[[]std.string](words), Count(words))
	fmt.Println(int(Last[[]std.string](words).size()), int(Last(words).size()))
	var g1 []std.string = Grow[std.string, []std.string](words, std.string("z"))
	var g2 []std.string = Grow[std.string](words, std.string("z"))
	fmt.Println(len(g1), len(g2), int(g1[2].size()))
	vv := [][]std.vector[int]{{std.vector[int]{}}}
	fmt.Println(Count[[][]std.vector[int]](vv))

	m := map[int]std.string{1: "a", 2: "b"}
	fmt.Println(Keys(m), Keys[map[int]std.string](m))
	v, ok := Get[map[int]std.string](m, 2)
	fmt.Println(int(v.size()), ok)
	v2, ok2 := Get(m, 3)
	fmt.Println(int(v2.size()), ok2)
	var p1 map[int]std.string = Put(m, 5, std.string("q"))
	var p2 map[int]std.string = Put[map[int]std.string](m, 6, std.string("rr"))
	fmt.Println(len(p1), len(p2), int(p2[6].size()))
	byName := map[string]std.string{"x": "y"}
	fmt.Println(ByName(byName), ByName[map[string]std.string](byName))

	hs := []Holder{{n: 1}, {n: 2}}
	hm := map[string]Holder{"a": {n: 3}}
	fmt.Println(Count[[]Holder](hs), Count(hs), Vals[map[string]Holder](hm), Vals(hm), Last[[]Holder](hs).n)
}

// Output:
// 2 2
// 3 3
// 3 3 1
// 1
// 2 2
// 1 true
// 0 false
// 4 4 2
// 1 1
// 2 2 1 1 2
