// Opaque template arguments: constants, constant expressions, Go types,
// nested arrays, and a C++ template from a local header.
package main

import (
	"fmt"
	"strings"

	"./shapes.hpp"
	"<array>"
	"<bitset>"
	"<vector>"
)

const N = 4

type Cell struct{ alive bool }

func main() {
	var a std.array[int, 4]
	a[0] = 1
	var b std.array[byte, N*2]
	b.fill(0)
	bits := std.bitset[64]{}
	bits.set(3)
	var grid std.array[std.array[Cell, N], N]
	grid[1][2].alive = true
	sizes := [2]int{1, 2}
	var c std.array[float64, len(sizes)]
	c[1] = 2.5
	box := geo.Box[strings.Builder]{}
	ints := geo.Box[int]{value: 7}
	fmt.Println(a[0], int(b.size()), bits.count(), grid[1][2].alive, c[1], ints.get(), box.value.Len())
	vs := std.vector[std.array[int, 3]]{}
	vs.push_back(std.array[int, 3]{1, 2, 3})
	fmt.Println(vs[0][2])
}
