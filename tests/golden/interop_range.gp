// phase: 04

package main

import (
	"<algorithm>"
	"<vector>"
)

func squares(n int) std.vector[int] {
	var v std.vector[int]
	for i := 0; i < n; i++ {
		v.push_back(i * i)
	}
	return v
}

func main() {
	v := squares(4)
	sum := 0
	for i, x := range v { // a copy of each element, the index
		sum += int(i) + int(x)
	}
	for i := range v { // the index only
		sum += int(i)
	}
	for range std.vector[int](2, 7) { // a temporary container, no variables
		sum++
	}
	lo, hi := std.minmax(3, 1) // a pair of references: copies
	sum += int(lo) + int(hi)
	println(sum)
}
