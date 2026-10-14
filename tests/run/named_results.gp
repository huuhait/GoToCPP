// phase: 02

// Named results: zero values, a bare return, a return with values, results
// changed after an assignment, and a function with no results.
package main

import "fmt"

func divmod(a, b int) (q, r int) {
	q = a / b
	r = a % b
	return
}

func zeroes() (i int, f float64, s string, ok bool, u uint8) {
	return
}

func minMax(a, b, c int) (lo, hi int) {
	lo, hi = a, a
	if b < lo {
		lo = b
	}
	if c < lo {
		lo = c
	}
	if b > hi {
		hi = b
	}
	if c > hi {
		hi = c
	}
	return lo, hi
}

func early(n int) (result string) {
	result = "default"
	if n < 0 {
		return "negative"
	}
	if n == 0 {
		return
	}
	result = "positive"
	return
}

func say(s string) {
	if s == "" {
		return
	}
	fmt.Println("say:", s)
}

func three() (int, string, bool) {
	return 3, "three", true
}

func main() {
	q, r := divmod(17, 5)
	fmt.Println(q, r)
	fmt.Println(divmod(-17, 5))
	fmt.Println(zeroes())
	lo, hi := minMax(5, -2, 9)
	fmt.Println(lo, hi)
	fmt.Println(early(-1), early(0), early(1))
	say("")
	say("hi")
	n, s, b := three()
	fmt.Println(n, s, b)
	_, s2, _ := three()
	fmt.Println(s2)
}

// Output:
// 3 2
// -3 -2
// 0 0  false 0
// -2 9
// negative default positive
// say: hi
// 3 three true
// three
