// Func values and closures: a returned closure, a func parameter, closures over loop variables
// (Go 1.22 per-iteration copies), a shared captured variable, a stateful closure, a recursive
// func value, and a closure passed to a higher-order function.
package main

func makeAdder(d int) func(int) int {
	return func(x int) int { return x + d }
}

func apply(f func(int) int, x int) int { return f(x) }

func fib() func() int {
	a, b := 0, 1
	return func() int {
		r := a
		a, b = b, a+b
		return r
	}
}

func each(xs []int, f func(int)) {
	for _, v := range xs {
		f(v)
	}
}

func main() {
	add2 := makeAdder(2)
	println(add2(5), apply(add2, 10))
	var fns []func() int
	for i := 0; i < 3; i++ {
		fns = append(fns, func() int { return i })
	}
	println(fns[0](), fns[1](), fns[2]())
	x := 1
	inc := func() { x++ }
	inc()
	inc()
	println(x)
	f := fib()
	println(f(), f(), f(), f(), f())
	var fact func(int) int
	fact = func(n int) int {
		if n <= 1 {
			return 1
		}
		return n * fact(n-1)
	}
	println(fact(5))
	total := 0
	each([]int{1, 2, 3}, func(v int) { total += v })
	println(total)
}
