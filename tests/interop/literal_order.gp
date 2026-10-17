// phase: 04

// Go's order of evaluation in literals of C/C++ types (step 4.6): the calls in the elements run
// from left to right, also in a keyed literal whose keys are not in the fields' order (its values
// are set in the literal's order) and in the keys and values of a std::map's literal; a package
// variable is read after the calls, as gc reads it.
package main

import (
	"fmt"

	"<map>"
	"<string>"
	"<vector>"

	"./lits.h"
)

var calls int

func next() C.int {
	calls++
	fmt.Println("next", calls)
	return C.int(calls)
}

func key(s string) std.string {
	calls++
	fmt.Println("key", s, calls)
	return std.string(s)
}

func val() int {
	calls++
	fmt.Println("val", calls)
	return calls
}

var g = 0

func bump() int {
	g++
	return g
}

func main() {
	v := lits.Vec{next(), next()}
	w := lits.Vec{y: next(), x: next()}
	fmt.Println(int(v.x), int(v.y), int(w.x), int(w.y))
	b := lits.Box{max: lits.Vec{next(), next()}, min: lits.Vec{y: next(), x: next()}}
	fmt.Println(int(b.min.x), int(b.min.y), int(b.max.x), int(b.max.y))
	m := std.map[std.string, int]{key("a"): val(), key("b"): val()}
	fmt.Println(int(m.size()), int(m["a"]), int(m["b"]))
	g = 10
	gs := std.vector[int]{g, bump(), g}
	fmt.Println(int(gs[0]), int(gs[1]), int(gs[2]))
	fmt.Println(int(lits.Vec{next(), 7}.x), len([]C.int{next(), next()}), int(lits.vec_sum(lits.Vec{y: next(), x: next()})))
}

// Output:
// next 1
// next 2
// next 3
// next 4
// 1 2 4 3
// next 5
// next 6
// next 7
// next 8
// 8 7 5 6
// key a 9
// val 10
// key b 11
// val 12
// 2 10 12
// 11 11 11
// next 13
// next 14
// next 15
// next 16
// next 17
// 13 2 33
