// phase: 03

// copy copies min(len(dst), len(src)) elements and returns that number, also between
// overlapping parts of one array and from a string into a []byte; clear zeroes the elements
// of s[:len(s)].
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
	fmt.Println("]")
}

func main() {
	s := []int{1, 2, 3, 4, 5}
	n := copy(s[1:], s)
	show(s)
	fmt.Println(n)
	n = copy(s, s[2:])
	show(s)
	fmt.Println(n)
	d := make([]int, 2)
	fmt.Println(copy(d, s), copy(s, d), copy(d, []int(nil)), copy(d[:0], s))
	show(d)
	show(s)

	b := make([]byte, 3)
	fmt.Println(copy(b, "hello"), string(b))
	fmt.Println(copy(b[1:], "é"), b[1], b[2])

	strs := []string{"a", "b", "c"}
	copy(strs, strs[1:])
	fmt.Println(strs[0], strs[1], strs[2])

	clear(s[1:3])
	show(s)
	clear(strs)
	fmt.Println(strs[0] == "", strs[1] == "", strs[2] == "", len(strs))
	var empty []int
	clear(empty)
	ptrs := []*int{new(int)}
	clear(ptrs)
	fmt.Println(ptrs[0] == nil, len(ptrs))
}

// Output:
// [1 1 2 3 4]
// 4
// [2 3 4 3 4]
// 3
// 2 2 0 0
// [2 3]
// [2 3 4 3 4]
// 3 hel
// 2 195 169
// b c c
// [2 0 0 3 4]
// true true true 3
// true 1
