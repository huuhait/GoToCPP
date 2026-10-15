// phase: 03

// Audit case: a function with defers has one exit (the single-exit shape, Phase 03 README §2.8):
// every return, also in loops, switches and nested blocks, runs the defers once, after the
// result is set, and a deferred closure can change a named result after the return statement.
package main

import "fmt"

var log []string

func note(s string) { log = append(log, s) }

func find(xs []int, want int) (idx int, found bool) {
	defer func() {
		if !found {
			idx = -1
		}
		note(fmt.Sprint("find ", want, " -> ", idx))
	}()
	for i, x := range xs {
		switch {
		case x == want:
			return i, true
		case x > want:
			return 0, false
		}
	}
	return
}

func loop(n int) (total int) {
	for i := 0; i < n; i++ {
		defer func() { total += i }() // each iteration's own i (Go 1.22)
	}
	if n > 2 {
		return 100
	}
	return 0
}

func nested(k int) string {
	defer note("nested out")
	{
		if k == 0 {
			return "zero"
		}
		for {
			if k%2 == 0 {
				return "even"
			}
			k--
			if k < 0 {
				break
			}
		}
	}
	return "never"
}

func value() (s string) {
	s = "set"
	defer func() { s += "+defer" }()
	return s + "+return"
}

func main() {
	fmt.Println(find([]int{1, 3, 5, 7}, 5))
	fmt.Println(find([]int{1, 3, 5, 7}, 4))
	fmt.Println(find(nil, 1))
	fmt.Println(loop(2), loop(4))
	fmt.Println(nested(0), nested(3), nested(8))
	fmt.Println(value())
	for _, l := range log {
		fmt.Println(l)
	}
}

// Output:
// 2 true
// -1 false
// -1 false
// 1 106
// zero even even
// set+return+defer
// find 5 -> 2
// find 4 -> -1
// find 1 -> -1
// nested out
// nested out
// nested out
