// phase: 04

// Literals of C++ class types (step 4.6): a positional literal is C++'s braced initialization,
// so a std::vector gets its elements (an initializer_list), a std::pair its two values and a
// std::map its pairs; Go's keyed form {k: v} with keys that are not names is a std::map's pairs
// too (a std::multimap may have a key twice), and keys that are names are fields (std::pair's
// first and second; a named constant as a key goes in parentheses). An inner literal may
// leave its type out: C++ decides what it is.
package main

import (
	"fmt"

	"<map>"
	"<string>"
	"<utility>"
	"<vector>"

	"./lits.h"
)

type Point struct{ X, Y int }

func main() {
	v := std.vector[int]{5, 3, 8}
	empty := std.vector[int]{}
	fmt.Println(int(v.size()), int(v[2]), bool(empty.empty()))

	vs := std.vector[lits.Vec]{{1, 2}, lits.Vec{y: 4}, {}}
	grid := std.vector[std.vector[int]]{{1, 2}, {3}}
	pts := std.vector[Point]{{1, 2}, Point{Y: 9}}
	fmt.Println(int(vs.size()), int(vs[0].y), int(vs[1].y), int(vs[2].x))
	fmt.Println(int(grid.size()), int(grid[0].size()), int(grid[1][0]), int(pts[0].X), int(pts[1].Y))

	ages := std.map[std.string, int]{"bob": 31, "alice": 29}
	pairs := std.map[std.string, int]{{"carol", 35}}
	k := std.string("dave")
	byVar := std.map[std.string, int]{{k, 40}}
	fmt.Println(int(ages.size()), int(ages["bob"]), int(ages["alice"]), int(pairs["carol"]), int(byVar["dave"]))
	multi := std.multimap[int, int]{1: 2, 1: 3, 0: 4}
	const one = 1
	byConst := std.map[int, std.string]{(one): "one"} // (one): a key, not a field name
	fmt.Println(int(multi.size()), int(multi.count(1)), int(byConst[1].size()))
	boxes := std.map[C.int, lits.Box]{1: {{0, 0}, {2, 2}}, 2: {}}
	fmt.Println(int(lits.box_area(boxes[1])), int(lits.box_area(boxes[2])))

	pr := std.pair[int, std.string]{7, "seven"}
	kp := std.pair[C.int, C.double]{second: 2.5, first: 3}
	fmt.Println(int(pr.first), int(pr.second.size()), int(kp.first), float64(kp.second))
	s := std.string{"hi"}
	s += "!"
	fmt.Println(int(s.size()))
}

// Output:
// 3 8 true
// 3 2 4 0
// 2 2 3 1 9
// 2 31 29 35 40
// 3 2 3
// 4 0
// 7 5 3 2.5
// 3
