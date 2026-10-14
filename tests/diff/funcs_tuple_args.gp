// phase: 03

package main

import "fmt"

type IDs []int

func two() (int, int) { return 3, 4 }

func three() (string, int, int) { return "s", 5, 6 }

func ids() (IDs, int) { return IDs{1, 2}, 9 }

func sum(xs ...int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func tagged(tag string, xs ...int) string {
	fmt.Println(tag, len(xs), sum(xs...))
	return tag
}

func first(s []int, n int) int { return s[0] + n }

func main() {
	fmt.Println(sum(two()))
	fmt.Println(tagged(three()))
	fv := sum
	fmt.Println(fv(two()))
	g := func(s string, a, b int) string { return s + string(rune('0'+a+b-10)) }
	fmt.Println(g(three()))
	fmt.Println(first(ids()))
	var vf func(prefix string, rest ...int) int = func(prefix string, rest ...int) int { return len(prefix) + len(rest) }
	fmt.Println(vf(three()))
	defer fmt.Println("deferred", sum(two()))
	defer func() { fmt.Println("in defer") }()
	defer vf(three())
}
