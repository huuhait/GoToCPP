// phase: 09

// Iterator adapters written as generic functions over iter.Seq and iter.Seq2: Map, Filter, Take,
// Chain, Zip with iter.Pull, Enumerate, early break running deferred cleanups of the iterators,
// slices.Collect/Sorted, maps.Keys/Values, and a recursive tree walk through nested yield calls.
package main

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
)

func Count(from int) iter.Seq[int] {
	return func(yield func(int) bool) {
		defer fmt.Println("  count closed")
		for i := from; ; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func Map[T, U any](s iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func Filter[T any](s iter.Seq[T], p func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s {
			if p(v) && !yield(v) {
				return
			}
		}
	}
}

func Take[T any](s iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		i := 0
		for v := range s {
			if !yield(v) {
				return
			}
			if i++; i == n {
				return
			}
		}
	}
}

func Chain[T any](ss ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, s := range ss {
			for v := range s {
				if !yield(v) {
					return
				}
			}
		}
	}
}

func Enumerate[T any](s iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		i := 0
		for v := range s {
			if !yield(i, v) {
				return
			}
			i++
		}
	}
}

func Zip[A, B any](a iter.Seq[A], b iter.Seq[B]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		nb, stop := iter.Pull(b)
		defer stop()
		for x := range a {
			y, ok := nb()
			if !ok || !yield(x, y) {
				return
			}
		}
	}
}

type Tree[T any] struct {
	L, R *Tree[T]
	V    T
}

func (t *Tree[T]) All(yield func(T) bool) bool {
	if t == nil {
		return true
	}
	return t.L.All(yield) && yield(t.V) && t.R.All(yield)
}

func (t *Tree[T]) Seq() iter.Seq[T] { return func(yield func(T) bool) { t.All(yield) } }

func main() {
	evens := Filter(Count(1), func(i int) bool { return i%2 == 0 })
	sq := Map(evens, func(i int) string { return fmt.Sprint(i * i) })
	fmt.Println(slices.Collect(Take(sq, 4)))
	for i, s := range Enumerate(Take(Count(10), 3)) {
		fmt.Println(i, s)
	}
	fmt.Println(slices.Collect(Chain(Take(Count(0), 2), slices.Values([]int{100, 200}), Take(Count(50), 1))))
	for a, b := range Zip(slices.Values([]string{"x", "y", "z"}), Map(Count(1), func(i int) float64 { return float64(i) / 2 })) {
		fmt.Println(a, b)
	}
	m := map[string]int{"b": 2, "a": 1, "c": 3}
	fmt.Println(slices.Sorted(maps.Keys(m)), slices.Sorted(maps.Values(m)))
	keys := slices.Collect(maps.Keys(m))
	slices.Sort(keys)
	fmt.Println(keys, strings.Join(slices.Collect(Map(slices.Values(keys), strings.ToUpper)), ","))
	t := &Tree[int]{&Tree[int]{nil, nil, 1}, &Tree[int]{&Tree[int]{nil, nil, 3}, nil, 4}, 2}
	for v := range t.Seq() {
		fmt.Print(v, ";")
		if v == 3 {
			break
		}
	}
	fmt.Println()
	fmt.Println(slices.Collect(t.Seq()), slices.Collect(Map(t.Seq(), func(i int) int8 { return int8(i * 60) })))
	next, stop := iter.Pull(Take(Count(7), 2))
	a, ok1 := next()
	b, ok2 := next()
	c, ok3 := next()
	stop()
	fmt.Println(a, ok1, b, ok2, c, ok3)
	for k, v := range maps.All(map[int]int{1: 1}) {
		fmt.Println(k, v)
	}
	for i, v := range slices.Backward([]string{"p", "q"}) {
		fmt.Println(i, v)
	}
	m2 := maps.Collect(Enumerate(slices.Values([]string{"zero", "one"})))
	fmt.Println(m2)
}

// Output:
//   count closed
// [4 16 36 64]
// 0 10
// 1 11
// 2 12
//   count closed
//   count closed
//   count closed
// [0 1 100 200 50]
// x 0.5
// y 1
// z 1.5
//   count closed
// [a b c] [1 2 3]
// [a b c] A,B,C
// 1;2;3;
// [1 2 3 4] [60 120 -76 -16]
//   count closed
// 7 true 8 true 0 false
// 1 1
// 1 q
// 0 p
// map[0:zero 1:one]
