// phase: 03

// What closures capture: variables that never change are copied into the closure, the others
// are shared with it (boxed); a closure changes a variable of the function (also a field, an
// element of an array, a parameter and a named result); nested closures share one variable;
// each counter has its own variable; a closure reads a variable when it runs.
package main

import "fmt"

type acc struct {
	n    int
	hist [3]int
}

func counter() (func() int, func()) {
	n := 0
	return func() int { n++; return n }, func() { n = 100 }
}

func param(start int) func() int {
	return func() int {
		start *= 2
		return start
	}
}

func named() (total int) {
	add := func(x int) { total += x }
	add(3)
	add(4)
	return
}

func nested() int {
	x := 1
	f := func() func() int {
		x++
		return func() int {
			x *= 10
			return x
		}
	}
	g := f()
	fmt.Println(x, g(), x)
	return x
}

func main() {
	next, reset := counter()
	fmt.Println(next(), next(), next())
	reset()
	fmt.Println(next())
	other, _ := counter()
	fmt.Println(other(), next())

	p := param(3)
	fmt.Println(p(), p(), p())
	fmt.Println(named(), nested())

	// A value that never changes after its declaration is copied: same result.
	limit := 3
	under := func(v int) bool { return v < limit }
	fmt.Println(under(2), under(5))

	// A closure reads a variable when it runs.
	msg := "first"
	show := func() string { return msg }
	msg = "second"
	fmt.Println(show())

	// Fields and elements of a struct variable that a closure changes.
	var a acc
	record := func(v int) {
		a.hist[a.n%3] = v
		a.n++
	}
	for _, v := range []int{5, 6, 7, 8} {
		record(v)
	}
	fmt.Println(a.n, a.hist[0], a.hist[1], a.hist[2])

	// An array that a closure only reads is copied; a slice shares its elements anyway.
	arr := [3]int{1, 2, 3}
	sl := []int{1, 2, 3}
	read := func() int { return arr[0] + sl[0] }
	sl[0] = 100
	fmt.Println(read())
	setSl := func() { sl[1] = 50 }
	setSl()
	fmt.Println(sl[1])

	// A closure that shadows a captured variable.
	y := 5
	h := func() int {
		y := y * 2
		return y
	}
	fmt.Println(h(), y)
}

// Output:
// 1 2 3
// 101
// 1 102
// 6 12 24
// 20 20 20
// 7 20
// true false
// second
// 4 8 6 7
// 101
// 50
// 10 5
