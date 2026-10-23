// phase: 07

// Iterator adapters over C++ values: a generic function takes an iterator and a Go closure and returns an iterator, all three
// with the type argument that C++ deduces. The range variable can also be assigned (for s = range ...), a body can return, use
// labels and defer, and a generic function collects what a loop yields.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Seq[T any](xs []T) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for _, x := range xs {
			if !yield(x) {
				return
			}
		}
	}
}

func Filter[T any](seq func(yield func(T) bool), keep func(T) bool) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for x := range seq {
			if keep(x) && !yield(x) {
				return
			}
		}
	}
}

func Collect[T any](seq func(yield func(T) bool)) []T {
	var out []T
	for x := range seq {
		out = append(out, x)
	}
	return out
}

func First[T any](seq func(yield func(T) bool), zero T) T {
	for x := range seq {
		return x
	}
	return zero
}

func main() {
	words := []std.string{"a", "bcd", "ef", "ghij"}
	for s := range Filter(Seq(words), func(s std.string) bool { return s.size() > 1 }) {
		fmt.Print(int(s.size()), ";")
	}
	fmt.Println()

	var last std.string
	for last = range Seq(words) {
	}
	fmt.Println(int(last.size()))

	n := 0
	for range Seq(words) {
		n++
	}
	fmt.Println(n)

	got := Collect(Filter(Seq(words), func(s std.string) bool { return s.size() != 3 }))
	fmt.Println(len(got), int(got[2].size()))

	vecs := []std.vector[int]{{1, 2}, {3}}
	fmt.Println(int(First(Seq(vecs), std.vector[int]{}).size()))

outer:
	for s := range Seq(words) {
		for t := range Seq(words) {
			if t.size() > s.size() {
				continue outer
			}
			if s.size() == 4 {
				break outer
			}
			fmt.Print(int(s.size()), int(t.size()), ";")
		}
	}
	fmt.Println()

	for s := range Seq(words) {
		defer fmt.Println("deferred", int(s.size()))
	}
	fmt.Println("end")
}

// Output:
// 3;2;4;
// 4
// 4
// 3 4
// 2
// 1 1;3 1;3 3;3 2;2 1;
// end
// deferred 4
// deferred 2
// deferred 3
// deferred 1
