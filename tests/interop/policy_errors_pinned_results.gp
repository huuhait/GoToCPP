// phase: 04

// Pinned type, several results: a std::tuple can't hold an object that C++ can neither copy nor
// move, so a function that returns one with another value, and a := of such results, are errors
// at the .gp lines (a single result works: policy_rows.gp).
package main

import (
	"./objects.h"
)

func two() (objects.Pinned, int) {
	return objects.make_pinned(1), 2 // ERROR "C\+\+: .*(could not convert|no matching constructor|no matching function)"
}

func main() {
	a, b := two() // ERROR "C\+\+: .*(no matching constructor|no matching function)"
	_, _ = a, b
}
