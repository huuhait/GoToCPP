// phase: 02

// Functions and expressions in Phase 02: several results (std::tuple), named
// results with a bare return, a shadowed variable, Go's order of evaluation
// for calls (gp_ temporaries), parallel assignment, the integer helpers
// (gp::div, gp::rem, gp::shl), small integers that wrap, float to int, and
// fmt.
package main

import "fmt"

func divmod(a, b int) (q, r int) {
	q = a / b
	r = a % b
	return
}

func swap(a, b string) (string, string) {
	return b, a
}

func trace(name string, v int) int {
	fmt.Println("eval", name)
	return v
}

func sum(a, b, c int) int {
	return a + b + c
}

func main() {
	q, r := divmod(17, 5)
	x, y := swap("go", "cpp")
	fmt.Println(q, r, x+y, len(x+y))
	{
		q := q * 10
		fmt.Println(q)
	}

	fmt.Println(sum(trace("a", 1), trace("b", 2), trace("c", 3)))
	total := trace("d", 4) * trace("e", 5)
	x, y = y, x
	fmt.Println(total, x, y)

	a, b := -7, 2
	var small int8 = 127
	small++
	var u uint8 = 200
	u += 100
	f := 2.75
	fmt.Println(a/b, a%b, 1<<b, small, u, int(f*2))
	fmt.Printf("%d %s %t %v\n", q, x, q > r, f)
}
