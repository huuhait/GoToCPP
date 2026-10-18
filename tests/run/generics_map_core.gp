// phase: 07

// Operations on a type parameter whose core type is a map: M ~map[K]V.
package main

import "fmt"

type Counts map[string]int

func Clone[M ~map[K]V, K comparable, V any](m M) M {
	if m == nil {
		return nil
	}
	out := make(M, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func Merge[M ~map[K]V, K comparable, V any](dst, src M) M {
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func Has[M ~map[K]V, K comparable, V any](m M, k K) bool {
	_, ok := m[k]
	return ok
}

func Get[M ~map[K]V, K comparable, V any](m M, k K, def V) V {
	if v, ok := m[k]; ok {
		return v
	}
	return def
}

func Remove[M ~map[K]V, K comparable, V any](m M, ks ...K) int {
	n := 0
	for _, k := range ks {
		if _, ok := m[k]; ok {
			delete(m, k)
			n++
		}
	}
	return n
}

func Invert[K, V comparable](m map[K]V) map[V]K {
	out := map[V]K{}
	for k, v := range m {
		out[v] = k
	}
	return out
}

func Group[T any, K comparable](xs []T, key func(T) K) map[K][]T {
	out := map[K][]T{}
	for _, x := range xs {
		k := key(x)
		out[k] = append(out[k], x)
	}
	return out
}

func main() {
	c := Counts{"a": 1, "b": 2}
	d := Clone(c)
	d["a"] = 100
	fmt.Printf("%T %v %v\n", d, c, d)
	fmt.Println(Merge(Counts{"x": 1}, Counts{"y": 2, "x": 3}))
	fmt.Println(Has(c, "a"), Has(c, "z"), Get(c, "b", -1), Get(c, "q", -1), Clone(Counts(nil)) == nil)
	fmt.Println(Remove(d, "a", "q", "b"), len(d))
	fmt.Println(Invert(map[string]int{"one": 1, "two": 2}))
	g := Group([]string{"apple", "avocado", "banana", "blueberry", "cherry"}, func(s string) byte { return s[0] })
	fmt.Println(len(g), g['a'], g['b'], g['c'])
	fmt.Println(Group([]int{1, 2, 3, 4, 5, 6}, func(i int) bool { return i%2 == 0 }))
}

// Output:
// main.Counts map[a:1 b:2] map[a:100 b:2]
// map[x:3 y:2]
// true false 2 -1 true
// 2 0
// map[1:one 2:two]
// 3 [apple avocado] [banana blueberry] [cherry]
// map[false:[1 3 5] true:[2 4 6]]
