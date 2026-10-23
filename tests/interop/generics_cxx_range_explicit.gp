// phase: 07

// A range over an instance of a generic function with written C++ type arguments, which go/types cannot type: the call
// (Seq[std.string](words)), a variable that holds it (with := and with var), the instance itself as the iterator
// (Each[std.string]), with two yield parameters, and with a C struct. go/types types none of it, so a range variable is a
// C/C++ value for gocpp even when its type is a Go type (the int index of Indexed[std.string]): it is converted, int(i), as a
// C value is. An explicit instance that returns a C++ container is still a range over that container.
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

func Indexed[T any](xs []T) func(yield func(int, T) bool) {
	return func(yield func(int, T) bool) {
		for i, x := range xs {
			if !yield(i, x) {
				return
			}
		}
	}
}

func Each[T any](yield func(T) bool) {
	var zero T
	if !yield(zero) {
		return
	}
	yield(zero)
}

func Id[T any](x T) T { return x }

func main() {
	words := []std.string{"a", "bcd", "ef"}
	for s := range Seq[std.string](words) {
		fmt.Print(int(s.size()), ";")
	}
	fmt.Println()

	it := Seq[std.string](words)
	for s := range it {
		if s.size() == 3 {
			break
		}
		fmt.Print(int(s.size()), ";")
	}
	fmt.Println()

	var again = Seq[std.string](words)
	n := 0
	for range again {
		n++
	}
	fmt.Println(n)

	for s := range Each[std.string] {
		fmt.Print(int(s.size()), ";")
	}
	fmt.Println()

	for i, s := range Indexed[std.string](words) {
		fmt.Print(i, ":", int(s.size()), ";")
	}
	fmt.Println()

	sum := 0
	for p := range Seq[grf.Pt]([]grf.Pt{{1, 2}, {3, 4}}) {
		sum += int(p.x) + int(p.y)
	}
	fmt.Println(sum)

	total := 0
	for _, x := range Id[std.vector[int]](std.vector[int]{4, 5}) {
		total += int(x)
	}
	fmt.Println(total, firstLong(words))
	fmt.Println(sizes(1, words), sizes("x", words))
}

// sizes ranges over an explicit instance in a generic function and in a func literal in it.
func sizes[T any](tag T, words []std.string) int {
	n := 0
	for s := range Seq[std.string](words) {
		n += int(s.size())
	}
	count := func() int {
		m := 0
		for range Seq[std.string](words) {
			m++
		}
		return m
	}
	return n + count()
}

func firstLong(words []std.string) int {
	for i, s := range Indexed[std.string](words) {
		if s.size() > 1 {
			return int(i)
		}
	}
	return -1
}

// Output:
// 1;3;2;
// 1;
// 3
// 0;0;
// 0:1;1:3;2:2;
// 10
// 9 1
// 9 9
