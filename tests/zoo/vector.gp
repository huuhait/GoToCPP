// phase: 04

// <vector>: construction forms, push_back/emplace_back, at() (throws std::out_of_range, caught
// in a C++ helper-free way by Go's recover: see exception tests), data(), iterators and
// range over a vector of structs and of vectors.
package main

import (
	"fmt"

	"<vector>"
)

type P struct{ X, Y int }

func main() {
	v := std.vector[int]{3, 1, 2}
	v.push_back(10)
	v.emplace_back(11)
	w := std.vector[int](3, 7)
	fmt.Println(int(v.size()), int(v.back()), int(w[2]), int(v.capacity()) >= 5)

	sum := 0
	for _, x := range v {
		sum += int(x)
	}
	fmt.Println("sum", sum)

	ps := std.vector[P]{{1, 2}, {3, 4}}
	ps.push_back(P{5, 6})
	for i, p := range ps {
		fmt.Println(int(i), p.X, p.Y)
	}

	grid := std.vector[std.vector[int]](2, std.vector[int](2, 1))
	grid[1][0] = 5
	fmt.Println(int(grid[0][0]), int(grid[1][0]), int(grid[1].size()))

	d := v.data()
	fmt.Println(int(d[1]), bool(v.empty()))
	v.erase(v.begin())
	v.insert(v.begin()+1, 99)
	fmt.Println(int(v[0]), int(v[1]), int(v[2]))
}

// Output:
// 5 11 7 true
// sum 27
// 0 1 2
// 1 3 4
// 2 5 6
// 1 5 2
// 1 false
// 1 99 2
