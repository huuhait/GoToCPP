// phase: 02

// Boolean operators: && and || stop early (shown by functions that print),
// ! and comparisons of bools.
package main

import "fmt"

func t(name string) bool {
	fmt.Print(name, " ")
	return true
}

func f(name string) bool {
	fmt.Print(name, " ")
	return false
}

func main() {
	fmt.Println(t("a") && f("b") && t("c"))
	fmt.Println(f("a") && t("b"))
	fmt.Println(t("a") || t("b"))
	fmt.Println(f("a") || f("b") || t("c"))
	fmt.Println(f("a") || t("b") && f("c"))
	fmt.Println(!t("a") || !f("b"))
	x := 5
	ok := x > 3 && x < 10
	fmt.Println(ok, !ok, ok == true, ok != false, ok == (x == 5))
	if f("cond1") || t("cond2") {
		fmt.Println("taken")
	}
	n := 0
	for t("loop") && n < 2 {
		n++
	}
	fmt.Println(n)
	var zero bool
	fmt.Println(zero, !zero && !zero)
}

// Output:
// a b false
// a false
// a true
// a b c true
// a b c false
// a b true
// true false true true true
// cond1 cond2 taken
// loop loop loop 2
// false true
