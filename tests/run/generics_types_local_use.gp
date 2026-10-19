// phase: 07

// Local types of generic functions used as struct, list node, map key, type switch case and sorting wrapper, and returned in an interface.
package main

import "fmt"

func Group[K comparable, V any](vs []V, key func(V) K) map[K][]V {
	type bucket struct {
		k  K
		vs []V
	}
	m := map[K]*bucket{}
	var order []*bucket
	for _, v := range vs {
		k := key(v)
		b := m[k]
		if b == nil {
			b = &bucket{k: k}
			m[k] = b
			order = append(order, b)
		}
		b.vs = append(b.vs, v)
	}
	out := map[K][]V{}
	for _, b := range order {
		out[b.k] = b.vs
	}
	return out
}

func Chain[T any](vs ...T) []T {
	type node struct {
		v    T
		next *node
	}
	var head *node
	for i := len(vs) - 1; i >= 0; i-- {
		head = &node{vs[i], head}
	}
	var out []T
	for n := head; n != nil; n = n.next {
		out = append(out, n.v)
	}
	fmt.Printf("%T\n", head)
	return out
}

func Keys[T comparable](vs []T) int {
	type key struct{ a, b T }
	seen := map[key]bool{}
	for i := 0; i+1 < len(vs); i++ {
		seen[key{vs[i], vs[i+1]}] = true
	}
	return len(seen)
}

func Kind[T any](v any) string {
	type wrap struct{ v T }
	switch v.(type) {
	case wrap:
		return "wrap"
	case T:
		return "T"
	}
	return "other"
}

func Mk[T any](v T) any {
	type wrap struct{ v T }
	return wrap{v}
}

func Sorted[T any](vs []T, less func(a, b T) bool) []T {
	type byLess struct {
		vs   []T
		less func(a, b T) bool
	}
	b := byLess{append([]T(nil), vs...), less}
	for i := 1; i < len(b.vs); i++ {
		for j := i; j > 0 && b.less(b.vs[j], b.vs[j-1]); j-- {
			b.vs[j], b.vs[j-1] = b.vs[j-1], b.vs[j]
		}
	}
	return b.vs
}

func Counter[T any]() func(T) int {
	type state struct{ n int }
	s := &state{}
	return func(T) int { s.n++; return s.n }
}

func main() {
	g := Group([]int{1, 2, 3, 4, 5}, func(i int) bool { return i%2 == 0 })
	fmt.Println(g[true], g[false])
	fmt.Println(Chain(1, 2, 3), Chain("a"))
	fmt.Println(Keys([]int{1, 2, 1, 2, 1}), Keys([]string{"a", "a"}))
	fmt.Println(Kind[int](Mk(1)), Kind[int](3), Kind[string](3), Kind[int](Mk("s")))
	fmt.Println(Sorted([]int{3, 1, 2}, func(a, b int) bool { return a < b }))
	c := Counter[string]()
	c("a")
	fmt.Println(c("b"))
	fmt.Printf("%T\n", Mk(2.5))
}

// Output:
// [2 4] [1 3 5]
// *main.node[int]
// *main.node[string]
// [1 2 3] [a]
// 2 1
// other T other other
// [1 2 3]
// 2
// main.wrap[float64]
