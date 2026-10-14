// phase: 02

// Functions with several results, := with a variable that exists already,
// and f(g()) where g has several results.
package main

import "fmt"

func sumDiff(a, b int) (int, int) {
	return a + b, a - b
}

func swap(s, t string) (string, string) {
	return t, s
}

func main() {
	q, r := sumDiff(17, 5)
	fmt.Println(q, r)
	a, b := swap("left", "right")
	fmt.Println(a, b)
	q, c := sumDiff(q*10, 3)
	fmt.Println(q, c)
	x, y := swap(swap(a, b))
	fmt.Printf("%s %s %d %v\n", x, y, q+r, q > r)
	fmt.Println(sumDiff(100, 7))
}

// Output:
// 22 12
// right left
// 223 217
// right left 235 true
// 107 93
