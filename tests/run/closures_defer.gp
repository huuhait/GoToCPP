// phase: 03

// defer with func values and closures: a deferred closure changes a named result (the result
// is boxed and the epilogue returns the box's value), a deferred func value is evaluated at the
// defer statement (assigning the variable later doesn't change it), a deferred closure reads
// the variables when it runs, a closure with defers of its own, and a deferred call of a
// closure with arguments evaluated at the defer statement.
package main

import "fmt"

func double() (n int) {
	defer func() { n *= 2 }()
	n = 5
	return n + 1
}

func wrap() (s string, err int) {
	defer func() {
		if err != 0 {
			s = "failed: " + s
		}
	}()
	return "open", 3
}

func value() {
	f := func() { fmt.Println("first f") }
	defer f()
	f = func() { fmt.Println("second f") }
	x := 1
	defer func(v int) { fmt.Println("arg", v, "now", x) }(x)
	x = 2
}

func inner() int {
	work := func() (r int) {
		defer func() { r += 10 }()
		return 1
	}
	return work() + work()
}

func main() {
	fmt.Println(double())
	fmt.Println(wrap())
	value()
	fmt.Println(inner())
	for i := range 2 {
		defer func() { fmt.Println("main defer", i) }()
	}
}

// Output:
// 12
// failed: open 3
// arg 1 now 2
// first f
// 22
// main defer 1
// main defer 0
