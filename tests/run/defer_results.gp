// phase: 03

// Results and defers: a return sets the results first, then the deferred calls run, so a
// deferred call that changes a named result through a pointer changes what the function
// returns; a bare return returns the named results as the deferred calls left them; results
// without names can't be changed by a deferred call; several results, return f(), returns in
// loops and switches, and recursion, each call with its own defers.
package main

import "fmt"

func set(p *int, v int) { *p = v }

func add(p *int, v int) { *p += v }

func wrap(p *string, prefix string) { *p = prefix + *p }

func named() (n int) {
	defer set(&n, 42)
	return 7
}

func bare() (n int, s string) {
	defer add(&n, 1)
	defer wrap(&s, "wrapped: ")
	n, s = 10, "msg"
	return
}

func unnamed() int {
	x := 5
	defer set(&x, 99)
	return x
}

func pair(v int) (int, string) {
	defer fmt.Println("pair returns", v)
	if v < 0 {
		return -v, "neg"
	}
	return v, "pos"
}

func forward() (a int, b string) {
	defer add(&a, 1000)
	return pair(-3)
}

func find(xs []int, want int) (at int) {
	defer add(&at, 100)
	for i, x := range xs {
		if x == want {
			return i
		}
	}
	return -1
}

func kind(n int) (s string) {
	defer wrap(&s, "kind ")
	switch {
	case n < 0:
		return "neg"
	case n == 0:
		return "zero"
	}
	return "pos"
}

func fact(n int) (r int) {
	defer fmt.Println("fact", n)
	if n <= 1 {
		return 1
	}
	return n * fact(n-1)
}

func main() {
	fmt.Println(named())
	fmt.Println(bare())
	fmt.Println(unnamed())
	fmt.Println(pair(4))
	fmt.Println(forward())
	fmt.Println(find([]int{5, 6, 7}, 6), find(nil, 1))
	fmt.Println(kind(-1), kind(0), kind(2))
	fmt.Println(fact(4))
}

// Output:
// 42
// 11 wrapped: msg
// 5
// pair returns 4
// 4 pos
// pair returns -3
// 1003 neg
// 101 99
// kind neg kind zero kind pos
// fact 1
// fact 2
// fact 3
// fact 4
// 24
