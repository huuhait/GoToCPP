// Functions with several results: std::tuple, f(g()), return g(), and := with
// a variable that exists already.
package main

import "fmt"

func pair(a, b int) (int, int) {
	return b, a
}

func three() (string, float64, bool) {
	return "x", 2.5, true
}

func sum(a, b int) int {
	return a + b
}

func forward() (int, int) {
	return pair(1, 2)
}

func noResults(s string) {
	println("noResults", s)
}

func main() {
	a, b := pair(1, 2)
	fmt.Println(a, b)
	s, f, ok := three()
	fmt.Println(s, f, ok)
	fmt.Println(sum(pair(3, 4)))
	fmt.Println(forward())
	println(pair(5, 6))
	c, b := pair(b, a) // b exists: it is assigned
	fmt.Println(a, b, c)
	_, d := pair(7, 8)
	e, _ := pair(9, 10)
	fmt.Println(d, e)
	x, y := b, a // b and a are read before x and y exist
	fmt.Println(x, y)
	b, z := a, b // b exists: both values are read before b changes
	fmt.Println(b, z)
	noResults("done")
	fmt.Printf("%v and %d and %s and %t and %f %%\n", a, b, s, ok, f)
	// Wrong verbs: the error forms name the Go type of each constant.
	fmt.Printf("%f %f %f %s %d %t %d\n", 5, int8(5), uint(7), 5, "x", 'a', 2.5)
	println(5, uint8(200), 'a', 1.5)
}
