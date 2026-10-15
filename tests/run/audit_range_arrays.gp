// phase: 03

// Audit case: range over an array iterates over a copy when the body could change the array
// (Phase 03 README §2.3); range over a slice, over a pointer to an array, and over an array whose
// body changes nothing see the elements as they are.
package main

import "fmt"

type Grid [3][3]int

func main() {
	arr := [4]int{1, 2, 3, 4}
	for i, v := range arr {
		arr[3-i] = v * 10
		fmt.Print(v, " ")
	}
	fmt.Println(arr)

	s := []int{1, 2, 3, 4}
	for i, v := range s {
		s[3-i] = v * 10
		fmt.Print(v, " ")
	}
	fmt.Println(s)

	b := [4]int{1, 2, 3, 4}
	for i, v := range &b {
		b[3-i] = v * 10
		fmt.Print(v, " ")
	}
	fmt.Println(b)

	var g Grid
	for i := range g {
		for j := range g[i] {
			g[i][j] = i*3 + j
		}
	}
	for _, row := range g {
		row[0] = -1 // a copy of the row
		g[2][2] = 99
		fmt.Print(row, " ")
	}
	fmt.Println(g)

	// The array changes through a closure in the body: still the copy.
	c := [3]string{"a", "b", "c"}
	set := func(i int, v string) { c[i] = v }
	for i, v := range c {
		if i == 0 {
			set(2, "z")
		}
		fmt.Print(v)
	}
	fmt.Println(" ", c)

	// Assigning to the iteration variables changes nothing.
	for i, v := range [3]int{7, 8, 9} {
		i, v = 10, 20
		_, _ = i, v
	}
	sum := 0
	for _, v := range [3]int{7, 8, 9} {
		sum += v
	}
	fmt.Println(sum)
}

// Output:
// 1 2 3 4 [40 30 20 10]
// 1 2 20 10 [100 200 20 10]
// 1 2 20 10 [100 200 20 10]
// [-1 1 2] [-1 4 5] [-1 7 8] [[0 1 2] [3 4 5] [6 7 99]]
// abc  [a b z]
// 24
