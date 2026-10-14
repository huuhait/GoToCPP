// phase: 03

// Func values whose parameters are larger than 128 KiB: the C++ type takes such a parameter as a
// const reference (gp::func<R(const A&)>), and a func literal copies it into a box at its entry,
// so the callee still gets its own copy, as in Go.
package main

import "fmt"

func total(a [20000]int) int {
	t := 0
	for _, v := range a {
		t += v
	}
	return t
}

func main() {
	var a [20000]int
	a[5], a[19999] = 7, 3
	f := total
	g := func(b [20000]int) int {
		b[0] = 100
		return b[0] + b[5]
	}
	fmt.Println(f(a), g(a), a[0])
	fs := []func([20000]int) int{f, g}
	for _, h := range fs {
		fmt.Println(h(a))
	}
	k := func(n int, b [20000]int, s string) (int, string) { return n + b[19999], s + "!" }
	fmt.Println(k(1, a, "big"))
}

// Output:
// 10 107 0
// 10
// 107
// 4 big!
