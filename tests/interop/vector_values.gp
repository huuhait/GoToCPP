// phase: 04

// C++ class templates as gocpp types: their zero values (value-initialized), gocpp types as
// template arguments (a struct, a gocpp pointer), constant template arguments, member types,
// and C++ values passed to and returned from functions, in struct fields and in package
// variables. The values are used only through C++ functions (std.size, std.empty) here: the
// operators and the member calls come in step 4.4.
package main

import (
	"fmt"

	"<array>"
	"<bitset>"
	"<iterator>"
	"<string>"
	"<vector>"
)

// Corners holds Points in a C++ template by value: Point is defined before it in C++.
type Corners struct{ pts std.array[Point, 2] }

type Point struct{ X, Y int }

type Node struct{ next *Node }

type Bag struct {
	name  string
	items std.vector[Point]
	count C.int
}

var global std.vector[int]

func same(v std.vector[int]) std.vector[int] { return v }

func sizes(a std.vector[Point], b std.vector[*Node]) (C.size_t, C.size_t) {
	return std.size(a), std.size(b)
}

func main() {
	var v std.vector[int]
	var nodes std.vector[*Node]
	var pts std.vector[Point]
	var arr std.array[C.int, 4]
	var bits std.bitset[64]
	var it std.vector[int].iterator
	var s std.string
	var bag Bag
	var corners Corners
	bag.name = "bag"
	n, k := sizes(pts, nodes)
	fmt.Println(int(std.size(same(v))), bool(std.empty(v)), int(n), int(k))
	fmt.Println(int(std.size(arr)), int(std.size(bits)), int(std.size(s)), int(std.size(global)))
	fmt.Println(bag.name, int(std.size(bag.items)), int(bag.count), int(std.size(corners.pts)))
	_ = it
}

// Output:
// 0 true 0 0
// 4 64 0 0
// bag 0 0 2
