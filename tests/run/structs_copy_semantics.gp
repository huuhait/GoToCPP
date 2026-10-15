// phase: 03

// Struct values copy: assignment, passing, returning and storing in slices, maps and arrays
// copy the struct with its arrays; slices, maps and pointers inside are shared. Structs with
// arrays compare field by field.
package main

import "fmt"

type Inner struct{ v [2]int }

type S struct {
	id    int
	arr   [3]int
	in    Inner
	sl    []int
	m     map[string]int
	p     *Inner
	label string
}

func modify(s S) S {
	s.id = 99
	s.arr[0] = 99
	s.in.v[1] = 99
	s.sl[0] = 99
	s.m["k"] = 99
	s.p.v[0] = 99
	return s
}

func main() {
	a := S{id: 1, arr: [3]int{1, 2, 3}, sl: []int{1}, m: map[string]int{}, p: &Inner{}, label: "a"}
	b := a
	b.arr[1] = 20
	b.label = "b"
	fmt.Println(a.arr, b.arr, a.label, b.label)

	c := modify(a)
	fmt.Println(a.id, a.arr, a.in, a.sl, a.m, *a.p)
	fmt.Println(c.id, c.arr, c.in, c.sl, c.m, *c.p)

	list := []S{a}
	list[0].arr[2] = 30
	arr := [1]S{a}
	arr[0].in.v[0] = 5
	m := map[int]S{1: a}
	tmp := m[1]
	tmp.id = 7
	fmt.Println(a.arr, list[0].arr, arr[0].in, a.in, m[1].id, tmp.id)

	x := Inner{[2]int{1, 2}}
	y := Inner{[2]int{1, 2}}
	z := y
	z.v[1] = 3
	fmt.Println(x == y, y == z, [2]Inner{x, y} == [2]Inner{y, x})
}

// Output:
// [1 2 3] [1 20 3] a b
// 1 [1 2 3] {[0 0]} [99] map[k:99] {[99 0]}
// 99 [99 2 3] {[0 99]} [99] map[k:99] {[99 0]}
// [1 2 3] [1 2 30] {[5 0]} {[0 0]} 1 7
// true false true
