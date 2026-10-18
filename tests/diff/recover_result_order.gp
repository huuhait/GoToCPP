// phase: 05

package main

import "fmt"

func trace(s string) int {
	fmt.Println("eval", s)
	return len(s)
}

// return evaluates its operands, sets the results, and only then run the deferred calls.
func f() (a, b int) {
	defer func() {
		fmt.Println("deferred sees", a, b)
		a, b = b, a
	}()
	return trace("one"), trace("three")
}

func g() (r int) {
	defer func() {
		if e := recover(); e != nil {
			r = r + 100
		}
	}()
	r = 1
	defer func() { r += 10 }()
	panic("g")
}

func main() {
	fmt.Println(f())
	fmt.Println(g())
}
