// phase: 07

// Two nested range-over-func loops with labeled jumps: the yield functions of both loops are kept
// and called after each loop finished (inner first, then outer), and after both. Every call must
// panic with Go's message and run nothing of the bodies.
package main

import "fmt"

var outerYield func(int) bool
var innerYield func(string) bool

func outer(yield func(int) bool) {
	outerYield = yield
	for i := 1; i <= 3; i++ {
		if !yield(i) {
			return
		}
	}
}

func inner(yield func(string) bool) {
	innerYield = yield
	for _, s := range []string{"x", "y", "z"} {
		if !yield(s) {
			return
		}
	}
}

func try(name string, f func()) {
	defer func() { fmt.Println(name, "->", recover()) }()
	f()
}

func run() int {
	hits := 0
outerLoop:
	for i := range outer {
		for s := range inner {
			hits++
			if s == "y" && i == 1 {
				continue outerLoop
			}
			if i == 2 {
				break outerLoop
			}
		}
	}
	return hits
}

func main() {
	fmt.Println("hits", run())
	try("inner", func() { innerYield("q") })
	try("outer", func() { outerYield(7) })
	try("inner again", func() { innerYield("r") })
}

// Output:
// hits 3
// inner -> runtime error: range function continued iteration after whole loop exit
// outer -> runtime error: range function continued iteration after whole loop exit
// inner again -> runtime error: range function continued iteration after loop body panic
