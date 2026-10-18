// phase: 04

// <algorithm>: sort, reverse, find, count, min/max with typed constants (both arguments must
// have one type: std.max(n, int(0))), minmax (a pair of references: two variables copy them),
// binary_search and accumulate from <numeric>.
package main

import (
	"fmt"

	"<algorithm>"
	"<numeric>"
	"<vector>"
)

func main() {
	v := std.vector[int]{5, 2, 9, 1, 7, 2}
	std.sort(v.begin(), v.end())
	fmt.Println(int(v[0]), int(v[5]), std.binary_search(v.begin(), v.end(), 7))
	std.reverse(v.begin(), v.end())
	fmt.Println(int(v[0]), int(std.count(v.begin(), v.end(), 2)))
	it := std.find(v.begin(), v.end(), 9)
	fmt.Println(int(it-v.begin()), std.find(v.begin(), v.end(), 100) == v.end())

	n := 3
	fmt.Println(int(std.max(n, int(10))), int(std.min(2.5, 1.5)))
	lo, hi := std.minmax(3, 1)
	fmt.Println(int(lo), int(hi))
	fmt.Println(int(std.accumulate(v.begin(), v.end(), 0)))
}

// Output:
// 1 9 true
// 9 2
// 0 true
// 10 1
// 1 3
// 26
