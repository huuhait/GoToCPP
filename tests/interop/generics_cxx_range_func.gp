// phase: 07

// A range over a generic iterator instantiated with C++ type arguments: the type argument is deduced by the C++ compiler from the
// call, the loop variables get the C++ type of what the iterator yields, and break, continue, return and nested loops work as for
// Go types. The same iterators yield C values to a Go closure, and Wrap[T any, S ~[]T] (the element before the slice) deduces T.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

import "./generics_cxx_range_func.hpp"

func Seq[T any](xs []T) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for _, x := range xs {
			if !yield(x) {
				return
			}
		}
	}
}

func Seq2[T any](xs []T) func(yield func(int, T) bool) {
	return func(yield func(int, T) bool) {
		for i, x := range xs {
			if !yield(i, x) {
				return
			}
		}
	}
}

func Each[T any](xs []T, yield func(T) bool) {
	for _, x := range xs {
		if !yield(x) {
			return
		}
	}
}

func Wrap[T any, S ~[]T](xs S) []T {
	var out []T
	for _, x := range xs {
		out = append(out, x)
	}
	return out
}

func Count[T any](seq func(yield func(T) bool)) int {
	n := 0
	for range seq {
		n++
	}
	return n
}

func main() {
	words := []std.string{"a", "bcd", "ef"}
	for s := range Seq(words) {
		fmt.Print(int(s.size()), ";")
	}
	fmt.Println()

	for i, s := range Seq2(words) {
		if i == 0 {
			continue
		}
		fmt.Print(i, ":", int(s.size()), ";")
	}
	fmt.Println()

	vecs := []std.vector[int]{{1, 2}, {3}, {4, 5, 6}}
	total := 0
	for v := range Seq(vecs) {
		total += int(v.size())
		if total > 3 {
			break
		}
	}
	fmt.Println(total)

	pts := []grf.Pt{{1, 2}, {3, 4}}
	sum := 0
	for p := range Seq(pts) {
		sum += int(p.x) * int(p.y)
	}
	fmt.Println(sum)

	each := 0
	Each(pts, func(p grf.Pt) bool {
		each += int(p.x)
		return true
	})
	fmt.Println(each)

	fmt.Println(find(words, 3), find(words, 9))

	w := Wrap(words)
	fmt.Println(len(w), int(w[1].size()))
	wp := Wrap(pts)
	fmt.Println(len(wp), int(wp[1].y))

	for _, s := range words {
		for t := range Seq(words) {
			if s.size() == t.size() {
				fmt.Print("=")
			}
		}
	}
	fmt.Println()
	fmt.Println(Count(Seq(vecs)))
}

func find(words []std.string, n int) int {
	for i, s := range Seq2(words) {
		if int(s.size()) == n {
			return i
		}
	}
	return -1
}

// Output:
// 1;3;2;
// 1:3;2:2;
// 6
// 14
// 4
// 1 -1
// 3 3
// 2 4
// ===
// 3
