// phase: 03

package main

import "fmt"

type Handler func(int) string

type Pred func(int) bool

type Reducer func(acc, x int) int

func run(h Handler, xs ...int) {
	if h == nil {
		fmt.Println("no handler")
		return
	}
	for _, x := range xs {
		fmt.Print(h(x), ";")
	}
	fmt.Println()
}

func filter(xs []int, p Pred) []int {
	var out []int
	for _, x := range xs {
		if p(x) {
			out = append(out, x)
		}
	}
	return out
}

func fold(xs []int, r Reducer, init int) int {
	acc := init
	for _, x := range xs {
		acc = r(acc, x)
	}
	return acc
}

func main() {
	run(func(n int) string { return "x" }, 1, 2)
	var h Handler
	run(h, 1)
	h = func(n int) string {
		if n%2 == 0 {
			return "even"
		}
		return "odd"
	}
	run(h, 1, 2, 3)
	var plain func(int) string = h
	fmt.Println(plain(4), plain != nil)
	h = Handler(plain)
	run(h)
	evens := filter([]int{1, 2, 3, 4, 5, 6}, func(x int) bool { return x%2 == 0 })
	fmt.Println(len(evens), evens[0], evens[2])
	fmt.Println(fold([]int{1, 2, 3, 4}, func(a, x int) int { return a*10 + x }, 0))
	var r Reducer = func(a, x int) int { return a + x }
	fmt.Println(fold(evens, r, 100))
	hs := map[string]Handler{"d": func(n int) string { return "d" }}
	fmt.Println(hs["d"](1), hs["e"] == nil)
}
