// phase: 03

// A call with several results whose results Go converts implicitly (step 3.9): a, b = f(),
// return f() (also in a function with defers), and g(f()) (also deferred).
package main

import "fmt"

type IDs []int
type Point struct{ X, Y int }

func two() ([]int, struct{ X, Y int }) {
	fmt.Println("two")
	return []int{1, 2, 3}, struct{ X, Y int }{4, 5}
}

func named() (IDs, Point) {
	return two()
}

func deferred() (IDs, Point) {
	defer fmt.Println("deferred runs")
	return two()
}

func show(ids IDs, p Point) int {
	fmt.Println("show", len(ids), p)
	return len(ids) + p.X
}

func main() {
	ids, p := named()
	fmt.Println(len(ids), p)
	ids, p = deferred()
	fmt.Println(len(ids), p.Y)
	fmt.Println(show(two()))
	defer show(two())
	var q Point
	ids, q = two()
	fmt.Println(ids[2], q)
}
