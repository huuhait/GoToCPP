// phase: 04

// Composite literals of C types (step 4.6): a positional literal is C++'s braced initialization
// (aggregate initialization for a C struct), a keyed one sets the named fields in the literal's
// order (also bit-fields), the fields left out are zero, and an inner literal in a literal of a C type may leave
// its type out (C++ decides what it is). Untyped constants are C literals.
package main

import (
	"fmt"

	"./lits.h"
)

var origin lits.Vec = lits.Vec{y: 7}

var pv = lits.Vec{x: 6} // the literal gives the variable its type

func corner(x C.int) lits.Vec { return lits.Vec{x, x + 1} }

func main() {
	v := lits.Vec{1, 2}
	w := lits.Vec{y: 5}
	var z lits.Vec = lits.Vec{}
	fmt.Println(int(v.x), int(v.y), int(w.x), int(w.y), int(z.x), int(lits.vec_sum(lits.Vec{3, 4})))

	b := lits.Box{{0, 0}, {4, 3}}
	k := lits.Box{max: {2, 5}}
	n := lits.Box{min: lits.Vec{y: 1}, max: lits.Vec{x: 3, y: 4}}
	fmt.Println(int(lits.box_area(b)), int(lits.box_area(k)), int(lits.box_area(n)))

	c := lits.Color{r: 255, a: 1 << 7}
	it := lits.Item{name: "lamp", at: &v, weight: 1.5, color: lits.Color{1, 2, 'c', 4}}
	fmt.Println(int(c.r), int(c.g), int(c.a), int(it.name[1]), int(it.at.y), float64(it.weight), int(it.color.b))
	none := lits.Item{}
	fmt.Println(none.name == nil, none.at == nil, float64(none.weight))

	t := lits.Triple{{1, 2, 3}, 3}
	p := lits.struct_pair_s{second: 4, first: 9}
	fmt.Println(int(lits.triple_sum(t)), int(lits.pair_diff(p)), int(lits.pair_diff(lits.struct_pair_s{1, 5})))
	fmt.Println(int(origin.x), int(origin.y), int(corner(4).y), int(lits.Vec{8, 9}.y), int(pv.x))

	f := lits.Flags{c: 9, b: 5}
	g := lits.Flags{1, 2, 3}
	fmt.Println(int(lits.flags_digits(f)), int(lits.flags_digits(g)))

	v = lits.Vec{y: v.x, x: v.y} // the old v.x and v.y
	fmt.Println(int(v.x), int(v.y), int(it.at.x))
}

// Output:
// 1 2 0 5 0 7
// 12 10 9
// 255 0 128 97 2 1.5 99
// true true 0
// 6 5 -4
// 0 7 5 9 6
// 59 123
// 2 1 2
