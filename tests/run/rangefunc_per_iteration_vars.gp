// phase: 07

package main

import "fmt"

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func Pairs(n int) func(yield func(int, string) bool) {
	return func(yield func(int, string) bool) {
		for i := range n {
			if !yield(i, fmt.Sprint("s", i)) {
				return
			}
		}
	}
}

func main() {
	var fs []func() int
	var ps []*int
	for x := range Cnt(4) {
		fs = append(fs, func() int { return x * 10 })
		ps = append(ps, &x)
		x++ // changes this iteration's copy only
	}
	for _, f := range fs {
		fmt.Println(f())
	}
	for _, p := range ps {
		fmt.Println(*p)
	}
	var gs []func() string
	for i, s := range Pairs(3) {
		gs = append(gs, func() string { i += 100; return fmt.Sprint(i, s) })
	}
	for _, g := range gs {
		fmt.Println(g(), g())
	}
	// Assigning to the range variable does not change the iteration.
	for x := range Cnt(3) {
		x *= 7
		fmt.Println("x", x)
	}
	// With =, the variable is shared.
	var shared int
	var hs []func() int
	for shared = range Cnt(3) {
		hs = append(hs, func() int { return shared })
	}
	for _, h := range hs {
		fmt.Println("shared", h())
	}
}

// Output:
// 10
// 20
// 30
// 40
// 1
// 2
// 3
// 4
// 100s0 200s0
// 101s1 201s1
// 102s2 202s2
// x 0
// x 7
// x 14
// shared 2
// shared 2
// shared 2
