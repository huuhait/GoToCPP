// phase: 04

// <span>: a gocpp slice converts implicitly to std::span (a view of the same memory), size,
// indexing, subspan, front/back, and writes through the span are seen by the slice.
package main

import (
	"fmt"

	"<span>"
)

func total(s std.span[int]) int {
	n := 0
	for i := 0; i < int(s.size()); i++ {
		n += int(s[i])
	}
	return n
}

func main() {
	nums := []int{1, 2, 3, 4, 5}
	sp := std.span[int](nums)
	fmt.Println(int(sp.size()), int(sp.front()), int(sp.back()), total(sp))
	sp[0] = 100
	sub := sp.subspan(1, 3)
	fmt.Println(nums[0], int(sub.size()), int(sub[0]), total(sub))
	fmt.Println(total(std.span[int](nums[2:])), sp.empty())
}

// Output:
// 5 1 5 15
// 100 3 2 9
// 12 false
