// if and else if, with init statements whose variables shadow outer ones, and
// shadowing in nested blocks (x := x + 1 inside an if).
package main

import "fmt"

func f(n int) int {
	fmt.Println("f", n)
	return n * 10
}

func classify(n int) string {
	if n < 0 {
		return "negative"
	} else if n == 0 {
		return "zero"
	} else if n < 10 {
		return "small"
	} else {
		return "large"
	}
}

func main() {
	x := 1
	if x := x + 1; x > 1 {
		fmt.Println("init x", x)
		x := x * 10
		fmt.Println("inner x", x)
	} else {
		fmt.Println("else x", x)
	}
	fmt.Println("outer x", x)

	// The init of an else if runs only when the conditions before it are false,
	// and its variable is visible in the rest of the chain.
	if a := f(1); a > 100 {
		fmt.Println("a", a)
	} else if b := f(2); a+b > 100 {
		fmt.Println("a+b", a, b)
	} else if c := f(3); a+b+c > 50 {
		fmt.Println("a+b+c", a, b, c)
	} else {
		fmt.Println("none", a, b, c)
	}
	if y := f(4); y > 0 {
		fmt.Println("y", y)
	} else if y := y + 1; y > 0 { // never runs: the first is true
		fmt.Println("y2", y)
	}

	for i := -1; i <= 11; i += 6 {
		fmt.Println(i, classify(i))
	}

	// Shadowing in nested blocks.
	v := 5
	if v > 0 {
		v := v + 1
		if v > 5 {
			v := v * 2
			fmt.Println("v", v)
		}
		fmt.Println("v", v)
	}
	fmt.Println("v", v)
	if ok := v == 5; ok {
		fmt.Println("ok")
	}
	if true {
		fmt.Println("constant condition")
	}
	if !(v > 3 && v < 10) || v == 7 {
		fmt.Println("not")
	} else {
		fmt.Println("in range")
	}
}
