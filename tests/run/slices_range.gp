// phase: 03

// range over a slice: the slice is evaluated once, the loop covers its original length (also
// when the body appends to it or sets it to nil), a write to a later element in the body is
// seen, the range variables are copies of hidden counters (Go 1.22: assigning to them doesn't
// change the iteration, and each iteration has its own variable), and the forms with one, two
// or no variables, and with variables that exist.
package main

import "fmt"

var calls int

func get() []int {
	calls++
	return []int{1, 2, 3}
}

func main() {
	s := []int{1, 2, 3}
	for i, v := range s {
		if i == 0 {
			s = append(s, 4)
			s[2] = 30
		}
		fmt.Print(i, ":", v, " ")
	}
	fmt.Println(len(s), "elements")

	t := []int{1, 2, 3}
	for i, v := range t {
		if i == 0 {
			t[2] = 30 // seen: the same array
			t = nil   // not seen: the loop has the old slice
		}
		fmt.Print(v, " ")
	}
	fmt.Println(t == nil, "nil")

	for i := range get() {
		fmt.Print(i)
	}
	for range get() {
		fmt.Print(".")
	}
	fmt.Println(calls)

	for i, v := range []string{"a", "b"} {
		i = 10
		v += "!"
		fmt.Print(i, v, " ")
	}
	fmt.Println("done")

	var ptrs []*int
	for _, v := range []int{5, 6, 7} {
		ptrs = append(ptrs, &v)
	}
	*ptrs[0] = 50
	fmt.Println(*ptrs[0], *ptrs[1], *ptrs[2])

	var i, v int
	for i, v = range []int{7, 8, 9} {
	}
	fmt.Println(i, v)
	w := []int{0, 0, 0, 0}
	for i, w[i] = range []int{7, 8, 9} {
	}
	fmt.Println(i, w[0], w[1], w[2], w[3])

	var nilSlice []int
	for range nilSlice {
		fmt.Println("never")
	}
	grid := [][]int{{1, 2}, {3}}
	sum := 0
	for _, row := range grid {
		for _, x := range row {
			sum += x
		}
	}
	fmt.Println(sum)
}

// Output:
// 0:1 1:2 2:3 4 elements
// 1 2 30 true nil
// 012...2
// 10a! 10b! done
// 50 6 7
// 2 9
// 2 8 9 7 0
// 6
