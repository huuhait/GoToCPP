// phase: 07

// An iterator type with a closure field and methods, over slices and maps of different element types.
package main

import "fmt"

type Iter[T any] struct {
	next func() (T, bool)
}

func FromSlice[T any](xs []T) *Iter[T] {
	i := 0
	return &Iter[T]{func() (T, bool) {
		if i >= len(xs) {
			var z T
			return z, false
		}
		i++
		return xs[i-1], true
	}}
}

func (it *Iter[T]) Next() (T, bool) { return it.next() }

func (it *Iter[T]) Filter(p func(T) bool) *Iter[T] {
	return &Iter[T]{func() (T, bool) {
		for {
			v, ok := it.next()
			if !ok || p(v) {
				return v, ok
			}
		}
	}}
}

func (it *Iter[T]) Take(n int) *Iter[T] {
	return &Iter[T]{func() (T, bool) {
		if n <= 0 {
			var z T
			return z, false
		}
		n--
		return it.next()
	}}
}

func (it *Iter[T]) Collect() []T {
	var out []T
	for {
		v, ok := it.next()
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func main() {
	nums := FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8})
	fmt.Println(nums.Filter(func(i int) bool { return i%2 == 0 }).Take(3).Collect())
	words := FromSlice([]string{"go", "c++", "gc", "rust"})
	fmt.Println(words.Filter(func(s string) bool { return s[0] == 'g' }).Collect())
	e := FromSlice[float64](nil)
	_, ok := e.Next()
	fmt.Println(ok)
	fmt.Printf("%T\n", e)
}

// Output:
// [2 4 6]
// [go gc]
// false
// *main.Iter[float64]
