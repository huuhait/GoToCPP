// phase: 03

// &s[i] is a pointer into the slice's array that keeps the array alive: it sees (and makes)
// the slice's writes, it outlives the slice and the function that made it, and after an
// append that copies to a new array it still points into the old one, as in Go.
package main

import "fmt"

var g = []int{1, 2, 3}

func elem(n int) *int {
	s := make([]int, n)
	s[n-1] = n * 10
	return &s[n-1]
}

func main() {
	s := []int{1, 2, 3}
	p := &s[1]
	*p = 20
	s[1]++
	fmt.Println(*p, s[1])
	s = append(s[:3:3], 4) // a new array
	*p = 99
	fmt.Println(*p, s[1])
	s = nil
	*p++
	fmt.Println(*p)

	q := elem(3)
	*q += 1
	fmt.Println(*q)

	r := &g[0]
	*r = 100
	fmt.Println(g[0], *r == g[0])

	grid := [][]int{{1, 2}, {3, 4}}
	e := &grid[1][0]
	rowp := &grid[0]
	*e = 30
	(*rowp)[1] = 20
	*rowp = append(*rowp, 5)
	fmt.Println(grid[1][0], grid[0][1], len(grid[0]))
	pp := &[]int{7, 8}[1]
	fmt.Println(*pp)
	ps := &s
	*ps = append(*ps, 1)
	(*ps)[0] = 2
	fmt.Println(len(s), s[0])
}

// Output:
// 21 21
// 99 21
// 100
// 31
// 100 true
// 30 20 3
// 8
// 1 2
