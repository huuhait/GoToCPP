// phase: 03

// Audit case: what is exact in both Go and gocpp: len, the capacity of make and of three-index
// slices, cap >= len, and whether appends share the array. (The exact capacity after growth is
// checked in append_cap_growth.gp and the tests next to it.)
package main

import "fmt"

func main() {
	s := make([]int, 2, 5)
	fmt.Println(len(s), cap(s), cap(s[1:]), cap(s[1:3:4]), cap(s[:0]))

	var grow []string
	ok := true
	for i := 0; i < 2000; i++ {
		grow = append(grow, "x")
		if cap(grow) < len(grow) {
			ok = false
		}
	}
	fmt.Println(len(grow), ok)

	// Within the capacity, append writes into the same array.
	a := make([]int, 3, 10)
	b := append(a, 4)
	b[0] = 9
	fmt.Println(a[0], len(a), len(b), cap(b) >= len(b))

	// Past a three-index slice's capacity, append makes a new array.
	c := a[0:2:2]
	d := append(c, 5)
	d[0] = 7
	fmt.Println(a[0], c[0], d, cap(c))

	bs := []byte("hello")
	bs = append(bs, " world"...)
	fmt.Println(string(bs), len(bs), cap(bs) >= len(bs))

	var nilS []int
	fmt.Println(len(nilS), cap(nilS), nilS == nil, cap(append(nilS, 1)) >= 1)
}

// Output:
// 2 5 4 3 5
// 2000 true
// 9 3 4 true
// 9 9 [7 0 5] 2
// hello world 11 true
// 0 0 true true
