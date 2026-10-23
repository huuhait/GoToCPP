// phase: 09

// Types that take no memory in Go but hold a field in C++ ([3]struct{}, struct{ z [0]int }: 3 and
// 1 bytes there, divergence D15) are real objects in gocpp: make, append(s, make([]T, n)...)
// and slices.Grow allocate their bytes, with Go's lengths, capacities and zero values. (A length
// that can't be allocated, such as math.MaxInt, is Go's fatal "out of memory" in gocpp: Go
// allocates nothing.)
package main

import (
	"fmt"
	"slices"
)

type Z struct{ z [0]int }
type Arr [3]struct{}
type Pair struct {
	a Z
	b Arr
}

func main() {
	z := make([]Z, 1<<20)
	z[1<<20-1] = Z{}
	fmt.Println(len(z), cap(z), z[5] == Z{})
	a := make([]Arr, 1000, 1<<20)
	a[999] = Arr{}
	fmt.Println(len(a), cap(a), a[0] == Arr{}, len(a[1]))
	p := make([]Pair, 1000)
	p[3].b[2] = struct{}{}
	fmt.Println(len(p), cap(p), p[3] == Pair{})

	z2 := append(z[:10], make([]Z, 1000)...)
	fmt.Println(len(z2), cap(z2))
	z3 := append([]Z{{}}, make([]Z, 1<<20)...)
	fmt.Println(len(z3), cap(z3))
	a2 := append(a, make([]Arr, 1<<20)...)
	fmt.Println(len(a2), cap(a2), a2[len(a2)-1] == Arr{})
	p2 := append([]Pair(nil), make([]Pair, 1000)...)
	fmt.Println(len(p2), cap(p2))

	g := slices.Grow([]Z{{}, {}}, 1000)
	fmt.Println(len(g), cap(g))
	ga := slices.Grow(a[:0:0], 1<<20)
	fmt.Println(len(ga), cap(ga))
	gp := slices.Grow(p, 1000)
	fmt.Println(len(gp), cap(gp))
	for i := 0; i < 5; i++ {
		g = append(g, Z{})
	}
	fmt.Println(len(g), cap(g))
}

// Output:
// 1048576 1048576 true
// 1000 1048576 true 3
// 1000 1000 true
// 1010 1048576
// 1048577 1048577
// 1049576 1049576 true
// 1000 1000
// 2 1002
// 0 1048576
// 1000 2000
// 7 1002
