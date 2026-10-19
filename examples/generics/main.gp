// phase: 07

// Generics shows generic functions (Map, Filter, Reduce), constraints with
// type sets, a generic type (Stack[T]) with methods, a generic type alias,
// range-over-func iterators (iter.Seq, iter.Seq2) and Go's generic packages
// cmp, maps and slices.
package main

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
)

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Filter[T any](xs []T, keep func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

func Reduce[T, A any](xs []T, init A, f func(A, T) A) A {
	acc := init
	for _, x := range xs {
		acc = f(acc, x)
	}
	return acc
}

// Number is a constraint with a type set: every type whose underlying type
// is one of these.
type Number interface {
	~int | ~int8 | ~int64 | ~float64
}

func Sum[T Number](xs []T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// Celsius is in Number's type set, because its underlying type is float64.
type Celsius float64

type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

// Pop removes the top item. On an empty stack it returns the zero value of T.
func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}

func (s *Stack[T]) Len() int { return len(s.items) }

// All yields the items from the top down, each with its depth.
func (s *Stack[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := len(s.items) - 1; i >= 0; i-- {
			if !yield(len(s.items)-1-i, s.items[i]) {
				return
			}
		}
	}
}

// Count yields 0, 1, ..., n-1.
func Count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

// Fib yields Fibonacci numbers without end; the loop decides when to stop.
func Fib() iter.Seq[int] {
	return func(yield func(int) bool) {
		a, b := 0, 1
		for yield(a) {
			a, b = b, a+b
		}
	}
}

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

func (p Pair[K, V]) String() string { return fmt.Sprintf("%v=%v", p.Key, p.Val) }

// SortedPairs returns the entries of m sorted by key.
func SortedPairs[K cmp.Ordered, V any](m map[K]V) []Pair[K, V] {
	var out []Pair[K, V]
	for _, k := range slices.Sorted(maps.Keys(m)) {
		out = append(out, Pair[K, V]{k, m[k]})
	}
	return out
}

// Set is a generic type alias (Go 1.24).
type Set[T comparable] = map[T]struct{}

func main() {
	nums := []int{1, 2, 3, 4, 5, 6}
	squares := Map(nums, func(n int) int { return n * n })
	evens := Filter(squares, func(n int) bool { return n%2 == 0 })
	total := Reduce(evens, 0, func(acc, n int) int { return acc + n })
	labels := Map(nums, func(n int) string { return fmt.Sprint("#", n) })
	fmt.Println(squares, evens, total, labels)
	fmt.Println(Reduce(labels, "", func(acc, s string) string { return acc + s }))

	fmt.Println(Sum(nums), Sum([]float64{1.5, 2.25}), Sum([]Celsius{20, 21.5}))
	fmt.Println(Sum([]int8{100, 27}), Sum([]int8{100, 100})) // int8 wraps around, in generic code too
	fmt.Println(Max(3, 7), Max("pear", "apple"), Max(2.5, -1))

	var st Stack[string]
	for _, w := range []string{"a", "b", "c"} {
		st.Push(w)
	}
	for depth, v := range st.All() {
		fmt.Println("depth", depth, v)
	}
	top, _ := st.Pop()
	fmt.Println("popped", top, "left", st.Len())
	var empty Stack[float64]
	v, ok := empty.Pop()
	fmt.Println(v, ok)
	fmt.Printf("%T %T\n", st, &empty)

	for x := range Count(10) {
		if x == 3 {
			break
		}
		fmt.Println("count", x)
	}
	var fibs []int
	for f := range Fib() {
		if f > 100 {
			break
		}
		fibs = append(fibs, f)
	}
	fmt.Println(fibs, slices.Collect(Count(4)))

	ages := map[string]int{"bob": 31, "alice": 29, "carol": 35}
	pairs := SortedPairs(ages)
	fmt.Println(pairs)
	fmt.Printf("%T %v\n", pairs[0], pairs[0])

	words := []string{"pear", "fig", "banana", "kiwi", "apple"}
	slices.SortFunc(words, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(a), len(b)), cmp.Compare(a, b))
	})
	fmt.Println(words, slices.Index(words, "kiwi"), slices.Contains(words, "plum"))
	rev := slices.Clone(nums)
	slices.Reverse(rev)
	fmt.Println(slices.Max(nums), rev, nums, len(slices.Repeat(nums, 2)))

	seen := Set[string]{}
	for _, w := range []string{"go", "cpp", "go", "gp"} {
		seen[w] = struct{}{}
	}
	fmt.Println(len(seen), slices.Sorted(maps.Keys(seen)))
}
