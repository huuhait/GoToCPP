// phase: 07

// A package variable without a type declared from a call of an instance of a generic function whose type arguments are
// all written or Go types (var it = Seq[std.string](words), also two variables from two results, a slice core type
// that gives the element, a C++ map element): its type is the instance's result type. The variable is assigned again
// and ranged over.
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

func Id[T any](x T) T { return x }

func Two[T any](x T) (T, T) { return x, x }

func Last[S ~[]T, T any](xs S) T { return xs[len(xs)-1] }

func Pair[K comparable, V any](k K, v V) map[K]V { return map[K]V{k: v} }

var words = []std.string{"a", "bcd"}
var it = Seq[std.string](words)
var one = Id[std.string](std.string("xyz"))
var a, b = Two[std.string](std.string("ab"))
var last = Last[[]std.string](words)
var m = Pair[int, std.vector[int]](1, std.vector[int]{1, 2, 3})
var n = Id[int](4)

func main() {
	for s := range it {
		fmt.Println(int(s.size()))
	}
	fmt.Println(int(one.size()), int(a.size()), int(b.size()), int(last.size()), n)
	_ = m
	it = Seq[std.string](words[:1])
	for s := range it {
		fmt.Println(int(s.size()))
	}
}

// Output:
// 1
// 3
// 3 2 2 3 4
// 1
