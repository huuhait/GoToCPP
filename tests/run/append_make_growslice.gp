// phase: 09

// append(s, make([]T, n)...) makes no slice, as gc compiles it (extendslice): the new elements are
// zero values, also over old ones in place, the capacity is growslice's, a negative n is
// "makeslice: len out of range" and a length past what can be allocated "growslice: len out of
// range" (slices.Grow(s, math.MaxInt)). m[k] = append(m[k], make([]T, n)...) keeps the make, as
// gc does (its mapAppend copies the make first; also x.m[k] and p.m[k]): "makeslice" there.
package main

import (
	"fmt"
	"math"
	"slices"
)

type Ints []int

type holder struct{ m map[int][]int }

func grow[S ~[]E, E any](s S, n int) S {
	return append(s, make([]E, n)...)
}

func try(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name+":", r)
		}
	}()
	f()
}

func main() {
	a := []int{1, 2, 3, 4}
	b := append(a[:1], make([]int, 2)...)
	fmt.Println(a, b, len(b), cap(b))
	strs := []string{"a", "b", "c"}
	t := append(strs[:1], make([]string, 2)...)
	fmt.Printf("%q %q\n", strs, t)
	x := 7
	type pair struct {
		s string
		p *int
	}
	ps := []pair{{"x", &x}, {"y", &x}}
	ps2 := append(ps[:0], make([]pair, 2)...)
	fmt.Println(ps2[0].s == "", ps2[1].p == nil, ps[1].p == nil)
	for _, n := range []int{0, 1, 3, 5, 33, 100, 1000} {
		g := append([]int{9, 8, 7}, make([]int, n)...)
		bs := append([]byte("hey"), make([]byte, n)...)
		ss := append([]string{"q"}, make([]string, n)...)
		fmt.Println(n, len(g), cap(g), g[len(g)-1], len(bs), cap(bs), bs[len(bs)-1], len(ss), cap(ss))
	}
	var nilS []int
	fmt.Println(append(nilS, make([]int, 0)...) == nil)
	u8 := append(nilS, make([]int, uint8(3))...)
	fmt.Println(u8, cap(u8))
	y := grow(Ints{4}, 2)
	fmt.Printf("%T %v\n", y, y)
	h := slices.Grow([]int{1, 2}, 10)
	fmt.Println(h, len(h), cap(h))
	h2 := slices.Grow(h, 3)
	fmt.Println(len(h2), cap(h2), &h2[0] == &h[0])
	e := slices.Grow([]struct{}{{}}, math.MaxInt-1)
	fmt.Println(len(e), cap(e))

	n := -1
	var big uint64 = math.MaxInt + 1
	try("Grow MaxInt", func() { slices.Grow([]int{1}, math.MaxInt) })
	try("Grow MaxInt-1", func() { slices.Grow([]int{1}, math.MaxInt-1) })
	try("Grow MaxInt/8", func() { slices.Grow([]int(nil), math.MaxInt/8) })
	try("Grow -1", func() { slices.Grow([]int{1}, n) })
	try("Grow zero-size", func() { slices.Grow(make([]struct{}, math.MaxInt), 1) })
	try("append make -1", func() { _ = append([]int{1}, make([]int, n)...) })
	try("append make MaxInt", func() { _ = append([]int{1}, make([]int, math.MaxInt)...) })
	try("append make uint", func() { _ = append([]byte{1}, make([]byte, big)...) })
	s := []string{"a"}
	try("in place", func() { s = append(s, make([]string, math.MaxInt/16)...) })
	try("generic", func() { grow(Ints{1}, math.MaxInt) })
	m := map[int][]int{}
	k := 1 << 62
	try("map append", func() { m[0] = append(m[0], make([]int, k)...) })
	try("map append other", func() { m[0] = append(m[1], make([]int, k)...) })
	hx := holder{m: map[int][]int{}}
	phx := &hx
	try("field map append", func() { hx.m[k] = append(hx.m[k], make([]int, k)...) })
	try("pointer field map append", func() { phx.m[k+1] = append(phx.m[k+1], make([]int, k)...) })
	try("field map append other", func() { hx.m[k] = append(phx.m[0], make([]int, k)...) })
	hx.m[1] = append(hx.m[1], make([]int, 2)...)
	fmt.Println(hx.m)
	fmt.Println(s, len(m))
}

// Output:
// [1 0 0 4] [1 0 0] 3 4
// ["a" "" ""] ["a" "" ""]
// true true true
// 0 3 3 7 3 3 121 1 1
// 1 4 6 0 4 8 0 2 2
// 3 6 6 0 6 8 0 4 4
// 5 8 8 0 8 8 0 6 6
// 33 36 36 0 36 48 0 34 35
// 100 103 112 0 103 112 0 101 111
// 1000 1003 1024 0 1003 1024 0 1001 1023
// true
// [0 0 0] 3
// main.Ints [4 0 0]
// [1 2] 2 12
// 2 12 true
// 1 9223372036854775807
// Grow MaxInt: runtime error: growslice: len out of range
// Grow MaxInt-1: runtime error: growslice: len out of range
// Grow MaxInt/8: runtime error: growslice: len out of range
// Grow -1: cannot be negative
// Grow zero-size: runtime error: growslice: len out of range
// append make -1: runtime error: makeslice: len out of range
// append make MaxInt: runtime error: growslice: len out of range
// append make uint: runtime error: makeslice: len out of range
// in place: runtime error: growslice: len out of range
// generic: runtime error: growslice: len out of range
// map append: runtime error: makeslice: len out of range
// map append other: runtime error: growslice: len out of range
// field map append: runtime error: makeslice: len out of range
// pointer field map append: runtime error: makeslice: len out of range
// field map append other: runtime error: growslice: len out of range
// map[1:[0 0]]
// [a] 0
