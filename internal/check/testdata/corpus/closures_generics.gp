// Opaque values in closures, method values, generic functions and
// deferred calls.
package main

import (
	"fmt"
	"slices"

	"<vector>"
	"./shapes.hpp"
)

func apply[T any](x T, f func(T) T) T { return f(x) }

func first[T any](xs []T) T { return xs[0] }

type Bag struct {
	items std.vector[int]
}

func (b *Bag) Add(n int) { b.items.push_back(n) }

func (b *Bag) Len() int { return int(b.items.size()) }

func main() {
	v := std.vector[int]{1, 2}
	add := func(n int) {
		v.push_back(n)
	}
	add(3)
	size := func() int { return int(v.size()) }
	fmt.Println(size())
	b := &Bag{}
	push := b.Add
	push(4)
	fmt.Println(b.Len(), apply(2, func(n int) int { return n * b.Len() }))
	vecs := []geo.Vec{{x: 1}, {x: 2}}
	fmt.Println(first(vecs).x, len(vecs))
	ints := []int{3, 1}
	slices.Sort(ints)
	fmt.Println(slices.Contains(ints, int(v.size())))
	defer func() {
		fmt.Println(int(v.size()))
	}()
	go b.Add(5)
	counter := 0
	each := func(f func(int)) {
		for _, x := range v {
			f(int(x))
		}
	}
	each(func(n int) { counter += n })
	fmt.Println(counter)
}
