// phase: 03

// Audit case: Go converts implicitly between types that are not identical where assignability
// allows it (Phase 03 README §2.4): an unnamed value to a named type in assignments, arguments,
// results, composite-literal elements and map values; a func literal to a named func type.
package main

import "fmt"

type IDs []int
type Set map[string]bool
type Handler func(string) string
type Pair [2]float64
type Matrix [][]int

func (ids IDs) Sum() int {
	s := 0
	for _, x := range ids {
		s += x
	}
	return s
}

func (s Set) Has(k string) bool { return s[k] }

func apply(h Handler, v string) string { return h(v) }

func makeIDs() IDs { return []int{4, 5, 6} }

func pairs() []Pair { return []Pair{[2]float64{1, 2}, {3, 4}}[:1:1] }

type Config struct {
	ids  IDs
	set  Set
	h    Handler
	grid Matrix
}

func main() {
	var ids IDs = []int{1, 2, 3}
	ids = append(ids, 4)
	raw := []int{10, 20}
	ids2 := IDs(raw)
	ids2[0] = 11 // shares raw's array
	fmt.Println(ids.Sum(), makeIDs().Sum(), raw, ids2.Sum())

	set := Set(map[string]bool{"a": true})
	var s2 Set = map[string]bool{"b": true}
	fmt.Println(set.Has("a"), set.Has("b"), s2.Has("b"))

	up := func(s string) string { return s + "!" }
	fmt.Println(apply(up, "hi"), apply(func(s string) string { return "<" + s + ">" }, "x"))

	c := Config{ids: []int{7}, set: map[string]bool{"k": true}, h: up, grid: [][]int{{1}, {2, 3}}}
	fmt.Println(c.ids.Sum(), c.set.Has("k"), c.h("c"), len(c.grid[1]))

	m := map[string]IDs{"x": []int{1, 1}}
	m["y"] = []int{2, 2, 2}
	fmt.Println(m["x"].Sum()+m["y"].Sum(), pairs(), len(pairs()))

	hs := []Handler{up, func(s string) string { return s + s }}
	for _, h := range hs {
		fmt.Print(h("ab"), " ")
	}
	fmt.Println()
}

// Output:
// 10 15 [11 20] 31
// true false true
// hi! <x>
// 7 true c! 2
// 8 [[1 2]] 1
// ab! abab
