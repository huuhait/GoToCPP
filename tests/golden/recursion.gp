// phase: 07

// The functions that can recurse carry GP_RECURSIVE and start with GP_RECURSION_MARK()
// (internal/lower/recursion.go), so that an endless recursion overflows the stack in -release
// too: a function that calls itself, two that call each other, a method that calls itself, a
// generic function that calls itself, and one that calls itself from a func literal in its body.
// The others have neither: one that only calls a recursive function, and a leaf.
package main

type T struct{ n int }

func (t T) depth(n int) int {
	if n <= 0 {
		return t.n
	}
	return t.depth(n - 1)
}

func fact(n int) int {
	if n <= 1 {
		return 1
	}
	return n * fact(n-1)
}

func even(n int) bool {
	if n == 0 {
		return true
	}
	return odd(n - 1)
}

func odd(n int) bool {
	if n == 0 {
		return false
	}
	return even(n - 1)
}

func count[E any](xs []E) int {
	if len(xs) == 0 {
		return 0
	}
	return 1 + count(xs[1:])
}

func walk(n int) int {
	f := func() int { return walk(n - 1) }
	if n <= 0 {
		return 0
	}
	return f()
}

func leaf(n int) int { return n + 1 }

func caller() int { return fact(3) + leaf(1) }

func main() {
	println(T{1}.depth(2), fact(4), even(4), count([]int{1, 2}), walk(2), caller())
}
