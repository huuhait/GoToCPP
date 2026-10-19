// phase: 05

// A keyword followed by !( keeps its Go meaning (not): if!(a), return!(x), case!(b): and for!(a) are
// plain Go, and so are != and a prefix ! after a name or a space. Only name!( with a name that is not a
// keyword is a must call (docs/spec/language.md, "Reading !"). The program is valid Go: real Go
// runs this file itself.

package main

import "fmt"

func neither(a, b bool) string {
	if!(a || b) {
		return "neither"
	}
	return "some"
}

func negate(x bool) bool {
	return!(x)
}

func pick(a, b bool) string {
	switch {
	case!(a):
		return "not a"
	case!(b):
		return "a, not b"
	}
	return "both"
}

func count(limit int) int {
	n := 0
	done := false
	for!(done) {
		n++
		done = n >= limit
	}
	return n
}

// notes: names that are not keywords, before a != and before a prefix !.
func notes(ok bool, x, y int) {
	fmt.Println(x != y, x!=y, !ok, ok != (x == y))
	var f = func(b bool) bool { return !b }
	fmt.Println(f(!ok), f (ok))
}

func main() {
	fmt.Println(neither(false, false), neither(true, false))
	fmt.Println(negate(true), negate(false))
	fmt.Println(pick(false, true), pick(true, false), pick(true, true))
	fmt.Println(count(4))
	notes(true, 1, 2)
	notes(false, 3, 3)
}

// Output:
// neither some
// false true
// not a a, not b both
// 4
// true true false true
// true false
// false false true true
// false true
