// phase: 07

// Generic functions and generic types together: functions taking and returning instances, with inferred type arguments.
package main

import "fmt"

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func Entries[K comparable, V any](m map[K]V, less func(a, b K) bool) []Pair[K, V] {
	out := make([]Pair[K, V], 0, len(m))
	for k, v := range m {
		out = append(out, Pair[K, V]{k, v})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && less(out[j].Key, out[j-1].Key); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func Keys[K comparable, V any](ps []Pair[K, V]) []K {
	var ks []K
	for _, p := range ps {
		ks = append(ks, p.Key)
	}
	return ks
}

func Map[T, U any](s *Stack[T], f func(T) U) *Stack[U] {
	out := &Stack[U]{}
	for _, v := range s.items {
		out.Push(f(v))
	}
	return out
}

func Zip[A, B any](as []A, bs []B) []Pair[int, struct {
	A A
	B B
}] {
	var out []Pair[int, struct {
		A A
		B B
	}]
	for i := range min(len(as), len(bs)) {
		out = append(out, Pair[int, struct {
			A A
			B B
		}]{i, struct {
			A A
			B B
		}{as[i], bs[i]}})
	}
	return out
}

func main() {
	es := Entries(map[string]int{"b": 2, "a": 1, "c": 3}, func(a, b string) bool { return a < b })
	fmt.Println(es, Keys(es))
	s := &Stack[int]{}
	s.Push(1)
	s.Push(2)
	m := Map(s, func(i int) Pair[string, int] { return Pair[string, int]{fmt.Sprint("n", i), i} })
	fmt.Printf("%v %T\n", m.items, m)
	fmt.Println(Zip([]int{1, 2, 3}, []string{"x", "y"}))
}

// Output:
// [{a 1} {b 2} {c 3}] [a b c]
// [{n1 1} {n2 2}] *main.Stack[main.Pair[string,int]]
// [{0 {1 x}} {1 {2 y}}]
