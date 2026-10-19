// phase: 07

package main

import "fmt"

type pair struct{ a, b int }

func Seq2(n int) func(yield func(int, string) bool) {
	return func(yield func(int, string) bool) {
		for i := range n {
			if !yield(i+1, fmt.Sprint("v", i)) {
				return
			}
		}
	}
}

func Seq1(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i + 10) {
				return
			}
		}
	}
}

func main() {
	var x int
	var s string
	for x, s = range Seq2(3) {
		fmt.Println("in", x, s)
	}
	fmt.Println("after", x, s)

	arr := make([]int, 4)
	m := map[string]int{}
	i := 0
	for arr[i], s = range Seq2(3) {
		m[s] = arr[i]
		i++
	}
	fmt.Println(arr, m, i)

	var p pair
	for p.a = range Seq1(3) {
		p.b += p.a
	}
	fmt.Println(p)

	keys := make([]string, 3)
	vals := map[int]int{}
	j := 0
	for vals[j], keys[j] = range Seq2(3) {
		j++
	}
	fmt.Println(vals, keys, j)

	var iface any
	for iface = range Seq1(2) {
		fmt.Printf("%T %v\n", iface, iface)
	}
	var e fmt.Stringer
	_ = e
	for range Seq1(2) {
		x++
	}
	fmt.Println(x)
	var y int
	for y = range Seq1(0) {
		y = -1
	}
	fmt.Println("y", y)
}

// Output:
// in 1 v0
// in 2 v1
// in 3 v2
// after 3 v2
// [1 2 3 0] map[v0:1 v1:2 v2:3] 3
// {12 33}
// map[0:1 1:2 2:3] [v0 v1 v2] 3
// int 10
// int 11
// 5
// y 0
