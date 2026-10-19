// phase: 07

// Index, assignment, delete and comma-ok over a map type parameter with a core type.
package main

import "fmt"

type Names map[string]int

func Get[M ~map[K]V, K comparable, V any](m M, k K) (V, bool) {
	v, ok := m[k]
	return v, ok
}

func Put[M ~map[K]V, K comparable, V any](m M, k K, v V) { m[k] = v }
func Del[M ~map[K]V, K comparable, V any](m M, k K)      { delete(m, k) }
func Bump[M ~map[K]V, K comparable, V ~int | ~float64](m M, k K) {
	m[k]++
	m[k] += 2
}
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	var out []K
	for k := range m {
		out = append(out, k)
	}
	return out
}
func Make[M ~map[K]V, K comparable, V any](n int) M { return make(M, n) }
func Copy[M ~map[K]V, K comparable, V any](m M) M {
	out := make(M, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
func Zero[M ~map[K]V, K comparable, V any](m M, k K) V { return m[k] }

type Pt struct{ X, Y int }

func main() {
	n := Names{"a": 1}
	v, ok := Get(n, "a")
	fmt.Println(v, ok)
	v, ok = Get(n, "zz")
	fmt.Println(v, ok)
	Put(n, "b", 20)
	Bump(n, "b")
	Bump(n, "new")
	Del(n, "a")
	Del(n, "none")
	fmt.Println(n, len(n))
	keys := Keys(n)
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	fmt.Println(keys)
	m := Make[map[Pt]float64](4)
	Bump(m, Pt{1, 2})
	Bump(m, Pt{1, 2})
	fmt.Println(m, Zero(m, Pt{9, 9}), Zero(map[[2]int]string{{1, 2}: "x"}, [2]int{1, 2}))
	c := Copy(n)
	c["q"] = 1
	fmt.Println(len(n), len(c), Make[Names](0) != nil)
	var nilMap map[int]int
	_, ok = Get(nilMap, 3)
	fmt.Println(ok, Zero(nilMap, 1))
	defer func() { fmt.Println("recovered:", recover()) }()
	Put(nilMap, 1, 1)
}

// Output:
// 1 true
// 0 false
// map[b:23 new:3] 2
// [b new]
// map[{1 2}:6] 0 x
// 2 3 true
// false 0
// recovered: assignment to entry in nil map
