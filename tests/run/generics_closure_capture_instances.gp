// phase: 07

// A closure that changes a variable of a generic function (a range-over-func body here, or a function
// literal) shares the variable with it in every instance of the function: the analysis of which
// variables are boxed ran once for the first instance and was reused for the others, so the second
// instance copied the variable into the closure and lost the changes (slices.Collect of an iterator
// of ints was empty after one of strings, step 9.10).

package main

import (
	"fmt"
	"iter"
)

func values[S ~[]E, E any](s S) iter.Seq[E] {
	return func(yield func(E) bool) {
		for _, v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

func appendSeq[S ~[]E, E any](s S, seq iter.Seq[E]) S {
	for v := range seq {
		s = append(s, v)
	}
	return s
}

func count[T any](items []T) int {
	n := 0
	each := func(item T) {
		n++
	}
	for _, item := range items {
		each(item)
	}
	return n
}

type set[T comparable] struct {
	items []T
}

func (s *set[T]) addAll(extra []T) {
	total := 0
	add := func(item T) {
		s.items = append(s.items, item)
		total++
	}
	for _, item := range extra {
		add(item)
	}
	if total != len(extra) {
		panic("lost an update")
	}
}

func main() {
	fmt.Println(appendSeq([]string(nil), values([]string{"a", "b"})))
	fmt.Println(appendSeq([]int(nil), values([]int{1, 2})))
	fmt.Println(appendSeq([]float64{0.5}, values([]float64{1.5, 2.5})))
	fmt.Println(count([]string{"x", "y", "z"}), count([]int{1, 2}), count([]bool{true}))

	var strs set[string]
	var ints set[int]
	strs.addAll([]string{"p", "q"})
	ints.addAll([]int{7, 8, 9})
	fmt.Println(strs.items, ints.items)
}

// Output:
// [a b]
// [1 2]
// [0.5 1.5 2.5]
// 3 2 1
// [p q] [7 8 9]
