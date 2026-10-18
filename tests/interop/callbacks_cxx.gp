// phase: 04

// Callbacks for C++ (step 4.11): where a C++ API takes a std::function, a function pointer or a
// template callable, a gocpp func goes in directly: a function of the package, a func literal, a
// closure. A std::function keeps the closure alive (the copies share it), as long as the object
// that holds it lives.
package main

import (
	"fmt"

	"<algorithm>"
	"<vector>"
	"./cbcxx.hpp"
)

func triple(x C.int) C.int { return x * 3 }

func main() {
	fmt.Println(cbx.apply(triple, 4))
	offset := 10
	fmt.Println(cbx.apply(func(x C.int) C.int { return x + C.int(offset) }, 4))
	offset = 20
	fmt.Println(cbx.apply_fn(triple, 5), cbx.apply_fn(func(x C.int) C.int { return -x }, 5))
	fmt.Println(cbx.twice(triple, 2), cbx.twice(func(x C.int) C.int { return x + C.int(offset) }, 2))

	// The button keeps the closure after on_click returns.
	total := 0
	var b cbx.Button
	b.on_click(func(x C.int) { total += int(x) })
	b.click(5)
	b.click(6)
	fmt.Println(total)

	// A comparator for std::sort, a predicate for std::count_if.
	var v std.vector[int]
	for _, x := range []int{4, 8, 1, 9, 3} {
		v.push_back(x)
	}
	std.sort(v.begin(), v.end(), func(a, b int) bool { return a > b })
	limit := 3
	n := std.count_if(v.begin(), v.end(), func(x int) bool { return x > limit })
	fmt.Println(int(v[0]), int(v[4]), int(n))

	// std.function[func(C.int) C.int] is std::function<int(int)>: made from a func, assigned a
	// func, passed on, called from gocpp. A vector of them holds gocpp funcs of the same shape.
	h := std.function[func(C.int) C.int](triple)
	fmt.Println(int(h(5)), int(cbx.apply(h, 5)))
	var k std.function[func(C.int) C.int]
	k = func(x C.int) C.int { return x + C.int(offset) }
	fmt.Println(int(k(1)), int(cbx.twice(k, 1)))
	var hooks std.vector[func(int) int]
	hooks.push_back(func(x int) int { return x + offset })
	fmt.Println(hooks[0](1))

	// Strings cross as C++ values.
	fmt.Println(cbx.pick(func(s std.string) std.string { return s + s }))
}

// Output:
// 13
// 15
// 15 -5
// 18 42
// 11
// 9 1 3
// 15 16
// 21 41
// 21
// abcabc
