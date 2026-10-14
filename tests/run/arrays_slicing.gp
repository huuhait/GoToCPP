// phase: 03

// Slicing an array shares its memory: a[lo:hi], a[lo:hi:max], a[:] of a local array (boxed),
// of a package array and through a pointer; an append within the capacity writes into the
// array, and one past it copies.
package main

import "fmt"

var table = [5]int{1, 2, 3, 4, 5}

func sum(s []int) int {
	t := 0
	for _, v := range s {
		t += v
	}
	return t
}

func main() {
	a := [5]int{10, 20, 30, 40, 50}
	s := a[1:3]
	s[0] = 21
	fmt.Println(a[1], len(s), cap(s), sum(a[:]))
	s = append(s, 31) // room: writes a[3]
	fmt.Println(a[3], len(s))
	t := a[1:2:2]
	t = append(t, 99) // full: a new array
	t[0] = -1
	fmt.Println(a[1], a[2], t[0], t[1])
	all := a[:]
	all[4] = 0
	fmt.Println(a[4], len(a[2:]), len(a[:4]), cap(a[1:3:4]))

	g := table[2:]
	g[0] = 30
	table[4] = 50
	fmt.Println(table[2], g[2], sum(table[:]))
	p := &table
	ps := p[:2]
	ps[1] = 200
	fmt.Println(table[1], sum(p[3:]))

	var rows [2][3]int
	r := rows[1][:]
	r[2] = 7
	fmt.Println(rows[1][2], len(r))
	sl := [][2]int{{1, 2}, {3, 4}}
	e := sl[1][:]
	e[0] = 30
	fmt.Println(sl[1][0])
	var empty [0]int
	es := empty[:]
	fmt.Println(len(es), es == nil)
}

// Output:
// 21 2 4 151
// 31 3
// 21 30 -1 99
// 0 3 4 3
// 30 50 87
// 200 54
// 7 3
// 30
// 0 false
