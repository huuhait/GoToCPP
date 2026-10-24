// phase: 08
// flags: -release

// A small recursive method of a generic type is inline in the instance's header, and it carries
// GP_RECURSIVE (no tail calls: internal/lower/recursion.go) after inline: the GNU attribute
// spelling, which clang++ takes among the decl-specifiers ([[...]] there was an error).
package main

import "fmt"

type Box[T any] struct{ v T }

func (b Box[T]) Count(n int) int {
	if n == 0 {
		return 0
	}
	return b.Count(n-1) + 1
}

// Even and Odd are small (one statement): inline methods in the instance's header.
func (b Box[T]) Even(n uint) bool { return n == 0 || b.Odd(n-1) }

func (b Box[T]) Odd(n uint) bool { return n != 0 && b.Even(n-1) }

func depth[T any](xs []T) int {
	if len(xs) == 0 {
		return 0
	}
	return 1 + depth(xs[1:])
}

func main() {
	b := Box[int]{1}
	fmt.Println(b.Count(3), depth([]string{"a", "b"}), b.Even(10), b.Odd(10))
}

// Output:
// 3 2 true false
