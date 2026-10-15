// phase: 03

// Closures as generators and helpers: a counter factory (each closure its own state), a
// Fibonacci generator, memoization in a captured map, function composition, and a slice of
// closures built in a loop (each with its own loop variable, Go 1.22).
package main

import "fmt"

func counter(start, step int) func() int {
	n := start - step
	return func() int {
		n += step
		return n
	}
}

func fibGen() func() int {
	a, b := 0, 1
	return func() int {
		r := a
		a, b = b, a+b
		return r
	}
}

func memo(f func(int) int) (func(int) int, func() int) {
	cache := map[int]int{}
	calls := 0
	return func(n int) int {
			if v, ok := cache[n]; ok {
				return v
			}
			calls++
			v := f(n)
			cache[n] = v
			return v
		}, func() int {
			return calls
		}
}

func compose(fs ...func(int) int) func(int) int {
	return func(x int) int {
		for _, f := range fs {
			x = f(x)
		}
		return x
	}
}

func main() {
	c1, c2 := counter(0, 1), counter(100, -10)
	fmt.Println(c1(), c1(), c2(), c1(), c2())
	fib := fibGen()
	var fs []int
	for range 10 {
		fs = append(fs, fib())
	}
	fmt.Println(fs)
	sq, calls := memo(func(n int) int { return n * n })
	fmt.Println(sq(4), sq(4), sq(5), calls())
	inc := func(x int) int { return x + 1 }
	dbl := func(x int) int { return x * 2 }
	fmt.Println(compose(inc, dbl)(5), compose(dbl, inc)(5), compose()(7))
	var printers []func() string
	for i := 0; i < 3; i++ {
		printers = append(printers, func() string { return fmt.Sprint("p", i) })
	}
	for _, p := range printers {
		fmt.Print(p(), " ")
	}
	fmt.Println()
}

// Output:
// 0 1 100 2 90
// [0 1 1 2 3 5 8 13 21 34]
// 16 16 25 2
// 12 11 7
// p0 p1 p2
