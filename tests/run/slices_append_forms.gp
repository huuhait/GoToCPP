// phase: 03

// The forms of append: one element, several, none, a slice (s...), a string's bytes into a
// []byte, the slice itself (s = append(s, s...)), and overlapping (removing an element with
// append(s[:i], s[i+1:]...)). The growth is Go's formula, but only cap >= len is checked: Go's
// exact capacity depends on its own element sizes.
package main

import "fmt"

func show(s []int) {
	fmt.Print("[")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println("]", cap(s) >= len(s))
}

func main() {
	var s []int
	for i := range 10 {
		s = append(s, i)
	}
	show(s)
	s = append(s, 10, 11, 12)
	s = append(s)
	show(s)
	s = append(s[:2], s[3:]...)
	show(s)
	s = append(s[:0], s[len(s)-3:]...)
	show(s)
	s = append(s, s...)
	show(s)
	t := append([]int(nil), s[1:3]...)
	t[0] = -1
	show(t)
	show(s)

	b := append([]byte("go"), "pher"...)
	b = append(b, '!')
	b = append(b, ""...)
	fmt.Println(string(b), len(b))

	var strs []string
	strs = append(strs, "a", "b")
	strs = append(strs, strs...)
	strs[0] = "z"
	fmt.Println(len(strs), strs[0], strs[1], strs[2], strs[3])

	var grid [][]int
	grid = append(grid, []int{1}, nil)
	grid[1] = append(grid[1], 2, 3)
	grid = append(grid, grid[1][1:])
	fmt.Println(len(grid), len(grid[0]), len(grid[1]), grid[2][0])

	big := make([]int, 0, 1)
	prev := cap(big)
	grows := 0
	for i := range 100000 {
		big = append(big, i)
		if cap(big) != prev {
			grows++
			prev = cap(big)
		}
	}
	fmt.Println(len(big), big[99999], cap(big) >= len(big), grows > 5, grows < 60)
}

// Output:
// [0 1 2 3 4 5 6 7 8 9] true
// [0 1 2 3 4 5 6 7 8 9 10 11 12] true
// [0 1 3 4 5 6 7 8 9 10 11 12] true
// [10 11 12] true
// [10 11 12 10 11 12] true
// [-1 12] true
// [10 11 12 10 11 12] true
// gopher! 7
// 4 z b a b
// 3 1 2 3
// 100000 99999 true true true
