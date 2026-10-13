// phase: 03

// Closures shows counters made by closures, func values passed around and
// stored, Go 1.22 loop variables, and defer: last in, first out, arguments
// evaluated at the defer statement, and a defer that changes a named result.
package main

import "fmt"

// counter returns a func that counts. Each counter has its own n, which lives
// on after counter returns.
func counter(step int) func() int {
	n := 0
	return func() int {
		n += step
		return n
	}
}

// apply calls f on every element and returns the results.
func apply(xs []int, f func(int) int) []int {
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

// compose returns a func that runs f, then g.
func compose(f, g func(int) int) func(int) int {
	return func(x int) int { return g(f(x)) }
}

func sum(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

// trace prints "enter" now and returns the func that prints "leave".
func trace(name string) func() {
	fmt.Println("enter", name)
	return func() { fmt.Println("leave", name) }
}

// work shows the order of deferred calls. The named result is set by the
// return statement, then changed by a deferred closure.
func work() (result int) {
	defer trace("work")()
	defer func() { result *= 2 }()
	for i := 1; i <= 3; i++ {
		defer fmt.Println("deferred", i) // i is evaluated now
	}
	x := 10
	defer fmt.Println("x at defer:", x)
	x = 20
	fmt.Println("x at return:", x)
	return 21
}

func main() {
	a, b := counter(1), counter(10)
	fmt.Println(a(), a(), a(), b(), b(), a())

	double := func(x int) int { return x * 2 }
	inc := func(x int) int { return x + 1 }
	xs := []int{1, 2, 3, 4}
	fmt.Println(apply(xs, double), apply(xs, compose(double, inc)), apply(xs, compose(inc, double)))
	fmt.Println(sum(), sum(1, 2, 3), sum(xs...))

	// Func values in a map. Map order is not fixed, so walk the names in a
	// fixed order.
	ops := map[string]func(int, int) int{
		"add": func(x, y int) int { return x + y },
		"sub": func(x, y int) int { return x - y },
		"mul": func(x, y int) int { return x * y },
	}
	for _, name := range []string{"add", "mul", "sub", "div"} {
		if op, ok := ops[name]; ok {
			fmt.Println(name, op(7, 3))
		} else {
			fmt.Println(name, "missing")
		}
	}

	// Since Go 1.22 each iteration has its own i, so each closure keeps its own.
	var fns []func() int
	for i := 0; i < 3; i++ {
		fns = append(fns, func() int { return i * i })
	}
	var squares []int
	for _, f := range fns {
		squares = append(squares, f())
	}
	fmt.Println("squares:", squares)

	// A closure that changes a variable of main: both see the same total.
	total := 0
	add := func(n int) { total += n }
	for n := range 5 {
		add(n)
	}
	fmt.Println("total:", total)

	var nothing func()
	fmt.Println("nil func:", nothing == nil, "non-nil:", add != nil)

	fmt.Println("work returned", work())
}
