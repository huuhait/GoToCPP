// phase: 02

// Calls in statements run in Go's order: := and = with several values,
// return with several values, if and else-if conditions, loop conditions
// (each time), op-assign, ++ in for posts, switch tags and cases, and
// increments of a counter passed through results.
package main

import "fmt"

func tr(name string, v int) int {
	fmt.Print(name, " ")
	return v
}

func two() (int, int) {
	return tr("r1", 1), tr("r2", 2)
}

func next(counter int) (int, int) {
	fmt.Print("next(", counter, ") ")
	return counter + 1, counter * 10
}

func main() {
	a, b := tr("a", 1), tr("b", 2)
	fmt.Println(a, b)
	a, b = tr("b'", b), tr("a'", a)
	fmt.Println(a, b)
	c, d := two()
	fmt.Println(c, d)
	var e, f int = tr("e", 5), tr("f", 6)
	fmt.Println(e, f)
	if tr("if1", 1) > tr("if2", 2) {
		fmt.Println("no")
	} else if tr("elif1", 3) < tr("elif2", 4) {
		fmt.Println("else if taken")
	}
	for i := 0; tr("cond-a", i) < tr("cond-b", 2); i++ {
		fmt.Print("body ", i, " ")
	}
	fmt.Println()
	g := 10
	g += tr("g1", 1) * tr("g2", 2)
	fmt.Println(g)
	switch tr("tag", 2) + tr("tag2", 0) {
	case tr("case1", 1) + tr("case1b", 0):
		fmt.Println("one")
	case tr("case2", 2) * tr("case2b", 1):
		fmt.Println("two")
	}
	counter := 0
	var v1, v2, v3 int
	counter, v1 = next(counter)
	counter, v2 = next(counter)
	counter, v3 = next(counter)
	fmt.Println(counter, v1, v2, v3)
	x, y := tr("x", 1)+tr("x2", 1), tr("y", 2)*tr("y2", 2)
	fmt.Println(x, y)
	for j := range tr("range-count", 2) + tr("range-count2", 1) {
		fmt.Print("iter ", j, " ")
	}
	fmt.Println()
	k := 0
	for tr("while", k) < 2 {
		k++
	}
	fmt.Println(k)
}
