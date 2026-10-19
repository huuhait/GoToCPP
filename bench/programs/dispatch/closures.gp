// bench: closures
// category: dispatch
// size: 100000000
// smoke: 50000
// twin: yes
// about: closure calls, closure-captured counters and higher-order functions
package main

import "fmt"

func makeAdder(k int) func(int) int {
	return func(x int) int { return x + k }
}

func apply(f func(int) int, n, x int) int {
	for i := 0; i < n; i++ {
		x = f(x)
	}
	return x
}

func compose(f, g func(int) int) func(int) int {
	return func(x int) int { return g(f(x)) }
}

func main() {
	n := benchN
	count := 0
	counter := func() int {
		count++
		return count
	}
	var sum uint64
	for i := 0; i < n; i++ {
		sum += uint64(counter())
	}

	add3 := makeAdder(3)
	mul := func(x int) int { return x*5 + 1 }
	both := compose(add3, mul)
	sum += uint64(apply(add3, n, 0))
	sum += uint64(apply(both, n, 1) & 0xffffff)

	fs := []func(int) int{makeAdder(1), makeAdder(2), mul, add3}
	x := 0
	for i := 0; i < n; i++ {
		x = fs[i&3](x) & 0xfffff
	}
	fmt.Println(sum, count, x)
}
