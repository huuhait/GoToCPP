// Named results, bare returns, and parameters without names.
package main

import "fmt"

func divmod(a, b int) (q, r int) {
	q = 7
	r = a - b
	return
}

func early(n int) (s string, ok bool) {
	s = "early"
	return s, n > 0
}

func blank() (_ int, x string) {
	x = "blank"
	return
}

func unnamed(int, string) bool {
	return true
}

func blankParam(_ int, b bool) bool {
	return !b
}

func zero() (n int, f float64, s string, b bool) {
	return
}

func main() {
	fmt.Println(divmod(10, 3))
	fmt.Println(early(1))
	fmt.Println(blank())
	fmt.Println(unnamed(1, "x"), blankParam(1, false))
	fmt.Println(zero())
}
