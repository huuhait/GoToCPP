// bench: slices_sort
// category: data
// size: 1500000
// smoke: 3000
// twin: no
// about: slices.Sort and slices.BinarySearch over a generated []int
package main

import (
	"fmt"
	"slices"
)

func main() {
	xs := make([]int, benchN)
	seed := uint64(99)
	for i := range xs {
		seed = seed*6364136223846793005 + 1442695040888963407
		xs[i] = int(seed >> 40)
	}
	slices.Sort(xs)
	hits := 0
	for i := 0; i < benchN; i += 3 {
		if _, ok := slices.BinarySearch(xs, xs[i]+i%5); ok {
			hits++
		}
	}
	fmt.Println(xs[0], xs[benchN/2], xs[benchN-1], hits)
}
