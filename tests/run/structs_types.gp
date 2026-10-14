// phase: 03

// Struct types: anonymous ones (identical types are one type), types declared in functions
// (two functions may declare a type of the same name), blank fields (not compared), field tags,
// fields named like C++ keywords or like their type, and a struct larger than 128 KiB (kept off
// the C++ stack).
package main

import "fmt"

type Item struct {
	class string
	new   int
	Item  int
	_     int
	auto  bool
}

type Big struct {
	cells [1 << 15]int64
	used  int
}

func fill(b Big, n int) Big {
	for i := range n {
		b.cells[i] = int64(i * i)
	}
	b.used = n
	return b
}

func local() {
	type T struct{ a, b int }
	t := T{b: 2}
	fmt.Println(t, t == T{0, 2})
}

func local2() {
	type T struct{ s string }
	fmt.Println(T{"two"})
}

func main() {
	a := struct {
		X int
		Y string `json:"y"`
	}{1, "a"}
	b := struct {
		X int
		Y string `json:"y"`
	}{X: 2}
	b = a
	fmt.Println(a, b, a == b)
	pts := []struct{ x, y int }{{1, 2}, {3, 4}}
	pts[1].y = 40
	fmt.Println(pts[0], pts[1], len(pts))

	local()
	local2()

	i := Item{class: "c", new: 1, Item: 2, auto: true}
	j := i
	fmt.Println(i, i == j)

	var big Big
	big = fill(big, 4)
	fmt.Println(big.used, big.cells[3], big.cells[4])
}

// Output:
// {1 a} {1 a} true
// {1 2} {3 40} 2
// {0 2} true
// {two}
// {c 1 2 0 true} true
// 4 9 0
