// phase: 04
// ExitCode: 2
// Stderr: (?m)^panic: runtime error: index out of range \[5\] with length 3$

// gocpp literals with C/C++ parts (step 4.6): structs with C fields, and slices, arrays and maps
// of C values and C structs. They are gocpp values (Go's rules: copies, bounds checks); an inner
// literal of a C type may leave its type out, as in Go, and an untyped constant is a C literal.
// A literal in a deferred call's arguments is evaluated by the defer statement.
package main

import (
	"fmt"

	"<string>"
	"<vector>"

	"./lits.h"
)

type Shape struct {
	name string
	box  lits.Box
	n    C.int
	at   *lits.Vec
	tag  std.string
}

func area(s Shape) int { return int(lits.box_area(s.box)) }

var globals = []C.int{7, 8}
var globalArray = [...]C.int{1, 2}
var globalKeyed = [...]C.int{2: 7, 8}

func show(v lits.Vec) { fmt.Println("deferred", int(v.x), int(v.y)) }

func main() {
	s := Shape{name: "sq", box: lits.Box{{0, 0}, {3, 3}}, n: 4, at: nil, tag: std.string("t")}
	u := Shape{"line", lits.Box{max: {5, 1}}, 2, nil, std.string{}}
	fmt.Println(s.name, area(s), int(s.n), s.at == nil, int(s.tag.size()), u.name, area(u), int(u.n))

	xs := []C.int{1, 2, 3}
	arr := [...]C.double{0.5, 1.5}
	localArray := [...]C.int{1, 2}
	localKeyed := [...]C.int{2: 7, 8}
	fmt.Println("ellipsis", len(globalArray), int(globalArray[1]), len(globalKeyed), int(globalKeyed[0]), int(globalKeyed[2]), int(globalKeyed[3]))
	fmt.Println("local", len(localArray), int(localArray[1]), len(localKeyed), int(localKeyed[0]), int(localKeyed[2]), int(localKeyed[3]))
	var zero [3]C.int
	fixed := [3]C.int{2: 7}
	vecs := []lits.Vec{{1, 2}, {y: 4}, lits.Vec{5, 6}}
	grid := [][]C.int{{1}, {2, 3}}
	fmt.Println(len(xs), int(xs[2]), len(arr), float64(arr[1]), int(zero[0]), int(fixed[2]), len(vecs), int(vecs[1].y), int(grid[1][1]))

	byName := map[string]lits.Vec{"a": {1, 2}, "b": lits.Vec{y: 9}}
	counts := map[string]C.int{"x": 1, "y": 2}
	fmt.Println(len(byName), int(byName["b"].y), int(counts["y"]))

	shapes := []Shape{{name: "a", box: lits.Box{max: {1, 1}}}, {name: "b", n: 9}}
	fmt.Println(len(shapes), area(shapes[0]), int(shapes[1].n))

	px := &[]C.int{3, 4, 5}
	pa := &[2]lits.Vec{{y: 1}}
	fmt.Println(len(*px), int((*px)[2]), int(pa[0].y), len(globals), int(globals[1]))
	ptrs := []*[]C.int{{1, 2}}
	lists := []std.vector[int]{{1, 2}, {}}
	fmt.Println(len(*ptrs[0]), int(lists[0].size()), int(lists[1].size()))
	n := C.int(1)
	defer show(lits.Vec{x: n}) // the literal is evaluated by the defer statement, as in Go
	n = 5

	copied := vecs[0]
	copied.x = 100 // a copy, as in Go
	fmt.Println(int(vecs[0].x), int(copied.x))
	fmt.Println(int(vecs[5].x))
}

// Output:
// sq 9 4 true 1 line 5 2
// ellipsis 2 2 4 0 7 8
// local 2 2 4 0 7 8
// 3 3 2 1.5 0 7 3 4 3
// 2 9 2
// 2 1 9
// 3 5 1 2 8
// 2 2 0
// 1 100
// deferred 1 0
