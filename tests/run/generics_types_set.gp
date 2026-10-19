// phase: 07

// A generic type whose underlying type is a map: methods on it, conversions, and range.
package main

import "fmt"

type Set[T comparable] map[T]struct{}

func (s Set[T]) Add(vs ...T) {
	for _, v := range vs {
		s[v] = struct{}{}
	}
}

func (s Set[T]) Has(v T) bool {
	_, ok := s[v]
	return ok
}

func (s Set[T]) Union(o Set[T]) Set[T] {
	out := Set[T]{}
	for k := range s {
		out.Add(k)
	}
	for k := range o {
		out.Add(k)
	}
	return out
}

func (s Set[T]) Intersect(o Set[T]) Set[T] {
	out := Set[T]{}
	for k := range s {
		if o.Has(k) {
			out.Add(k)
		}
	}
	return out
}

func (s Set[T]) Len() int { return len(s) }

func FromSlice[T comparable](xs []T) Set[T] {
	s := make(Set[T], len(xs))
	s.Add(xs...)
	return s
}

func main() {
	a := FromSlice([]int{1, 2, 3})
	b := FromSlice([]int{3, 4})
	fmt.Println(a.Union(b).Len(), a.Intersect(b).Len(), a.Has(2), b.Has(2))
	s := Set[string]{}
	s.Add("x", "y", "x")
	fmt.Println(s.Len(), s.Has("y"), len(s))
	m := map[string]struct{}(s)
	fmt.Println(len(m))
	fmt.Printf("%T %v\n", a.Intersect(b), a.Intersect(b))
	var nilSet Set[float64]
	fmt.Println(nilSet.Len(), nilSet.Has(1.5), nilSet == nil)
}

// Output:
// 4 1 true false
// 2 true 2
// 2
// main.Set[int] map[3:{}]
// 0 false true
