// phase: 03

// Recursion through a variable: the closure captures the variable that holds it. The variable is
// boxed (captured and assigned), so the closure calls itself through the box. Such a closure and
// its box hold each other, a reference cycle that is never freed (D1): fine for a program, but
// LeakSanitizer reports it (a test that builds a cycle on purpose).
package main

import "fmt"

func main() {
	var fib func(int) int
	fib = func(n int) int {
		if n < 2 {
			return n
		}
		return fib(n-1) + fib(n-2)
	}
	fmt.Println(fib(20))

	var walk func(n, depth int) int
	walk = func(n, depth int) int {
		if n == 0 {
			return depth
		}
		return walk(n/2, depth+1)
	}
	fmt.Println(walk(1000, 0))

	// Two closures that call each other.
	var even, odd func(int) bool
	even = func(n int) bool { return n == 0 || odd(n-1) }
	odd = func(n int) bool { return n != 0 && even(n-1) }
	fmt.Println(even(10), odd(7), even(3))
}

// Output:
// 6765
// 10
// true true false
