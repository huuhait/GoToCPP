// return: one value, several values, a bare return with named results that the
// body changes, named results in every statement form, and functions that end
// with a statement that Go calls terminating (C++ must not warn about them).
package main

import "fmt"

func divmod(a, b int) (q, r int) {
	q = a / b
	r = a % b
	if r < 0 {
		r += b
		q--
	}
	return
}

func count(n int) (evens, odds int) {
	for i := range n {
		if i%2 == 0 {
			evens++
			continue
		}
		odds++
	}
	return
}

func swap(a, b string) (string, string) {
	return b, a
}

func early(n int) (s string, ok bool) {
	s = "small"
	if n > 10 {
		s = "big"
		return s, true // explicit values
	}
	switch {
	case n < 0:
		return "negative", false
	}
	return
}

func sign(x int) int {
	if x < 0 {
		return -1
	} else if x > 0 {
		return 1
	}
	return 0
}

func kind(s string) int {
	switch s {
	case "a":
		fallthrough
	case "b":
		return 1
	default:
		return 2
	}
}

func firstAbove(n, limit int) int {
	for {
		if n > limit {
			return n
		}
		n *= 2
	}
}

func must(ok bool) string {
	if ok {
		return "ok"
	}
	panic("not ok")
}

func ladder(n int) int {
	if n < 0 {
		return -1
	} else if n == 0 {
		return 0
	} else {
		switch {
		case n < 10:
			return 1
		default:
			return 2
		}
	}
}

func main() {
	fmt.Println(kind("a"), kind("b"), kind("c"))
	fmt.Println(firstAbove(3, 100), must(true))
	fmt.Println(ladder(-4), ladder(0), ladder(5), ladder(50))
	for i, b := 0, false; i < 2; i, b = i+1, !b {
		fmt.Println(i, b)
	}
	fmt.Println(divmod(17, 5))
	fmt.Println(divmod(-17, 5))
	fmt.Println(count(7))
	fmt.Println(swap("a", "b"))
	fmt.Println(early(20))
	fmt.Println(early(-3))
	fmt.Println(early(4))
	fmt.Println(sign(-5), sign(0), sign(9))
}
