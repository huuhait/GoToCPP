// Func values, closures and variadics (Phase 03 step 3.11): func types as gp::func, functions
// of the package as values (gp::func_of), nil and == nil, func values passed, returned and
// stored in slices, maps and struct fields, calls of them (gp_call where the call could change
// them), func literals as lambdas with their captures (by value when never changed, the box
// otherwise), nested closures, captured parameters, named results and loop variables (Go 1.22:
// one box per iteration), recursion through a variable, literals called at once, deferred
// literals that change a named result, deferred func values, closures in package variables,
// variadic functions (no arguments, several, f(s...) sharing the slice, f(g()) with g's results),
// named func types (a func literal passed as one), and the order of a func value and its
// arguments.
package main

import "fmt"

var greeting = func(name string) string { return "hello, " + name }

var counterAt = func(start int) func() int {
	n := start
	return func() int {
		n++
		return n
	}
}(10)

var handler func(int) int

type Visitor func(int) bool

func visit(xs []int, v Visitor) int {
	n := 0
	for _, x := range xs {
		if v == nil || v(x) {
			n++
		}
	}
	return n
}

func pair() (int, int) { return 5, 6 }

type ops struct {
	name string
	f    func(int, int) int
}

func double(x int) int { return x * 2 }

func apply(xs []int, f func(int) int) []int {
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func compose(f, g func(int) int) func(int) int {
	return func(x int) int { return g(f(x)) }
}

func adder(step int) (func(), func() int) {
	total := 0
	return func() { total += step }, func() int { return total }
}

func sum(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

func join(sep string, parts ...string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += sep
		}
		s += p
	}
	return s
}

func push(s []int, xs ...int) []int {
	for _, x := range xs {
		s = append(s, x)
	}
	return s
}

func zero(xs ...int) {
	for i := range xs {
		xs[i] = 0
	}
}

func named() (r int) {
	defer func() { r *= 2 }()
	return 21
}

func deferred() {
	f := func(s string) { fmt.Println("deferred", s) }
	defer f("first")
	f = func(s string) { fmt.Println("changed", s) }
	defer f("second")
}

func print3(xs []int) {
	for _, x := range xs {
		fmt.Print(x, " ")
	}
	fmt.Println()
}

func setHandler() int {
	handler = func(x int) int { return x + 100 }
	return 1
}

func main() {
	fmt.Println(greeting("gocpp"), counterAt(), counterAt())

	inc, get := adder(3)
	inc()
	inc()
	fmt.Println("total", get())

	print3(apply([]int{1, 2, 3}, double))
	print3(apply([]int{1, 2, 3}, compose(double, func(x int) int { return x + 1 })))

	var fn func()
	fmt.Println(fn == nil, inc != nil, nil == get)

	table := map[string]func(int, int) int{
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b int) int { return a * b },
	}
	fmt.Println(table["add"](3, 4), table["mul"](3, 4), table["sub"] == nil)
	list := []ops{{"add", table["add"]}, {"sub", func(a, b int) int { return a - b }}}
	for _, o := range list {
		fmt.Println(o.name, o.f(10, 3))
	}

	// Go 1.22: each iteration has its own i, and a closure keeps it.
	var fns []func() int
	for i := 0; i < 3; i++ {
		fns = append(fns, func() int { return i * i })
	}
	for _, v := range []int{7, 8} {
		fns = append(fns, func() int { return v })
	}
	for _, f := range fns {
		fmt.Print(f(), " ")
	}
	fmt.Println()

	// A closure that changes a variable of main, and one that only reads one.
	count, limit := 0, 3
	bump := func() bool {
		count++
		return count < limit
	}
	for bump() {
	}
	fmt.Println("count", count)

	// Nested closures: the inner one changes a variable of main.
	outer := func(k int) func() {
		return func() { count += k }
	}
	outer(5)()
	fmt.Println("count", count)

	// Recursion through a variable.
	var fib func(int) int
	fib = func(n int) int {
		if n < 2 {
			return n
		}
		return fib(n-1) + fib(n-2)
	}
	fmt.Println("fib", fib(10))

	// A literal called at once, with arguments and results.
	q, r := func(a, b int) (int, int) { return a / b, a % b }(17, 5)
	fmt.Println(q, r)

	fmt.Println(named())
	deferred()

	// Variadics.
	nums := []int{4, 5, 6}
	fmt.Println(sum(), sum(1), sum(1, 2, 3), sum(nums...), join("-", "a", "b", "c"), join(","))
	zero(nums...)
	fmt.Println(nums[0], nums[1], nums[2])
	grown := push(nil, 1, 2)
	fmt.Println(len(grown), grown[0], grown[1])
	vf := sum
	fmt.Println(vf(7, 8))

	fmt.Println(sum(pair()), visit(nums, nil), visit([]int{1, 2, 3}, func(x int) bool { return x > 1 }))
	var vis Visitor = func(x int) bool { return x == 2 }
	fmt.Println(visit([]int{2, 2, 3}, vis), vis(3))

	// The func value is read after the calls of its arguments, as gc does.
	handler = func(x int) int { return x }
	fmt.Println(handler(setHandler()))
}
