// phase: 07

// Map keys and set membership with comparable type parameters instantiated with interfaces and structs.
package main

import "fmt"

type Key struct {
	A string
	B int
}
type Color int

func (c Color) String() string { return [...]string{"red", "green"}[c] }

type Stringer interface{ String() string }

func Set[K comparable](xs ...K) map[K]struct{} {
	m := map[K]struct{}{}
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}

func Count[K comparable](xs []K) map[K]int {
	m := map[K]int{}
	for _, x := range xs {
		m[x]++
	}
	return m
}

func Has[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

func Invert[K, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

func Lookup[K comparable, V any](m map[K]V, k K, def V) V {
	if v, ok := m[k]; ok {
		return v
	}
	return def
}

func main() {
	s := Set[any](1, "1", 1.0, int8(1), Key{"a", 1}, Key{"a", 1}, [2]int{1, 2}, [2]int{1, 2}, nil, nil, true, Color(1))
	fmt.Println(len(s), Has(s, any(1)), Has(s, any(int64(1))), Has(s, any(Key{"a", 1})), Has(s, any(Key{"a", 2})), Has(s, any([2]int{1, 2})), Has(s, nil), Has(s, any(Color(1))), Has(s, any(1)))
	c := Count([]Stringer{Color(0), Color(1), Color(0), nil})
	fmt.Println(len(c), c[Color(0)], c[Color(1)], c[nil], c[Color(5)])
	k := Count([]Key{{"a", 1}, {"b", 1}, {"a", 1}})
	fmt.Println(k[Key{"a", 1}], k[Key{"b", 1}], k[Key{"c", 1}], len(k))
	inv := Invert(map[string]Color{"r": 0, "g": 1})
	fmt.Println(inv[0], inv[1], len(Invert(map[int]int{1: 5, 2: 5})))
	fmt.Println(Lookup(map[Key]string{{"x", 1}: "found"}, Key{"x", 1}, "none"), Lookup(map[any]int{1: 10}, any("1"), -1), Lookup(map[[2]string]bool{}, [2]string{}, true))
	p, q := new(int), new(int)
	ptrs := Count([]*int{p, q, p, nil, nil})
	fmt.Println(len(ptrs), ptrs[p], ptrs[q], ptrs[nil])
	defer func() { fmt.Println("recovered:", recover()) }()
	Set[any](1, []int{2})
}

// Output:
// 9 true false true false true true true true
// 3 2 1 1 0
// 2 1 0 2
// r g 1
// found -1 true
// 3 2 1 2
// recovered: runtime error: hash of unhashable type []int
