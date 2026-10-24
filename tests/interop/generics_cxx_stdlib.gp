// phase: 08

// The generic functions of the standard library with a C++ element type (std.string), deduced and
// written: slices.Insert and Replace (make(S, n) of an S ~[]E that C++ deduces, also inside
// append(s, make(S, n)...)), Grow, Clone, Concat, Repeat, Compact, Delete, DeleteFunc, Reverse, Clip,
// Index, Equal and SortFunc; Replace and Insert without values (the empty variadic of a deduced E is
// {}, which leaves E to its default), Clone of an empty slice that is not nil (S{} of a deduced S is
// the empty []E, not nil); maps.Clone (the runtime copies a gocpp map of C++ values that an any
// holds), and sync.OnceValue and OnceValues, also when f panics; conversions of what an instance that
// go/types could not make returns (int(v.size()) of v := num(), num a sync.OnceValue of a std.string).
package main

import (
	"fmt"
	"maps"
	"slices"
	"sync"

	"<string>"
)

func Ins[S ~[]E, E any](s S, v ...E) S { return append(s, v...) }

func Lit[S ~[]E, E any](s S) S { return S{} }

func show(xs []std.string) {
	fmt.Print(len(xs), ":")
	for _, x := range xs {
		fmt.Print(" ", x)
	}
	fmt.Println()
}

func main() {
	c := []std.string{"a", "b", "b"}
	c = slices.Insert(c, 1, std.string("q"), std.string("r"))
	show(c)
	c = slices.Insert(c, len(c), std.string("end"))
	show(c)
	c = slices.Insert[[]std.string](c, 0, std.string("first"))
	show(c)
	c = slices.Replace(c, 0, 2, std.string("p"))
	show(c)
	c = slices.Delete(c, 0, 1)
	show(c)
	c = slices.Compact(c)
	show(c)
	c = slices.DeleteFunc(c, func(s std.string) bool { return s.size() > 2 })
	show(c)

	g := slices.Grow(c, 10)
	fmt.Println(len(g), cap(g) >= len(c)+10)
	d := slices.Clone(c)
	d[0] = std.string("zz")
	show(c)
	show(d)
	show(slices.Concat(c, d))
	show(slices.Repeat(c, 2))
	show(slices.Clip(c))
	slices.Reverse(d)
	show(d)
	slices.SortFunc(d, func(a, b std.string) int { return int(a.compare(b)) })
	show(d)
	fmt.Println(slices.Index(d, std.string("q")), slices.Contains(d, std.string("none")), slices.Equal(c, c))

	m := map[int]std.string{1: "x", 2: "yy"}
	m2 := maps.Clone(m)
	m2[3] = std.string("zzz")
	m2[1] = std.string("changed")
	fmt.Println(len(m), len(m2), m[1], m2[1], m2[3])
	m3 := maps.Clone[map[int]std.string](m)
	fmt.Println(len(m3), m3[2])
	var none map[int]std.string
	fmt.Println(maps.Clone(none) == nil)

	calls := 0
	once := sync.OnceValue(func() std.string { calls++; return std.string("once") })
	fmt.Println(once(), once(), calls)
	written := sync.OnceValue[std.string](func() std.string { return std.string("written") })
	fmt.Println(written())
	two := sync.OnceValues(func() (std.string, int) { calls++; return std.string("two"), calls })
	s, n := two()
	s2, n2 := two()
	fmt.Println(s, n, s2, n2)
	num := sync.OnceValue(func() std.string { return std.to_string(42) })
	v := num()
	fmt.Println(int(v.size()), int(num().size()), any(v) != nil)
	ins := slices.Insert(c, 0, std.string("i"))
	fmt.Println(int(ins[0].size()), ins[0] == std.string("i"), ins[1])
	e := []std.string{"a", "b", "c"}
	e = slices.Replace(e, 0, 1)
	show(e)
	e = slices.Insert(e, 0)
	var vals []std.string
	e = slices.Insert(e, 1, vals...)
	show(Ins(e))
	empty := e[2:]
	var none2 []std.string
	fmt.Println(empty == nil, slices.Clone(empty) == nil, Lit(e) == nil, len(Lit(e)), slices.Clone(none2) == nil)
	bad := sync.OnceValue(func() std.string { panic("no value") })
	for range 2 {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			fmt.Println(bad())
		}()
	}
}

// Output:
// 5: a q r b b
// 6: a q r b b end
// 7: first a q r b b end
// 6: p q r b b end
// 5: q r b b end
// 4: q r b end
// 3: q r b
// 3 true
// 3: q r b
// 3: zz r b
// 6: q r b zz r b
// 6: q r b q r b
// 3: q r b
// 3: b r zz
// 3: b r zz
// -1 false true
// 2 3 x changed zzz
// 2 yy
// true
// once once 1
// written
// two 2 two 2
// 2 2 true
// 1 true q
// 2: b c
// 2: b c
// false false false 0 true
// recovered: no value
// recovered: no value
