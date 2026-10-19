// phase: 07

// A type argument that is an interface satisfies comparable, but comparing two uncomparable dynamic values panics.
package main

import "fmt"

type Box struct{ V any }

func Eq[T comparable](a, b T) bool { return a == b }

func Has[T comparable](xs []T, x T) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func Count[K comparable](keys []K) int {
	m := map[K]int{}
	for _, k := range keys {
		m[k]++
	}
	return len(m)
}

func try(name string, f func() any) {
	defer func() { fmt.Println(name, "recovered:", recover()) }()
	fmt.Println(name, f())
}

func main() {
	try("same type, comparable", func() any { return Eq[any](1, 1) })
	try("different types", func() any { return Eq[any]([]int{1}, 1) })
	try("slice vs slice", func() any { return Eq[any]([]int{1}, []int{1}) })
	try("map vs map", func() any { return Eq[any](map[int]int{}, map[int]int{}) })
	try("func vs func", func() any { return Eq[any](func() {}, func() {}) })
	try("struct holding slice", func() any { return Eq[Box](Box{[]int{1}}, Box{[]int{1}}) })
	try("struct holding int", func() any { return Eq[Box](Box{1}, Box{1}) })
	try("array of any", func() any { return Eq[[2]any]([2]any{1, []int{}}, [2]any{1, []int{}}) })
	try("array of any, differ first", func() any { return Eq[[2]any]([2]any{1, []int{}}, [2]any{2, []int{}}) })
	try("has", func() any { return Has([]any{1, "a", []int{1}}, any("a")) })
	try("has, reaches slice", func() any { return Has([]any{1, []int{1}}, any([]int{1})) })
	try("map key", func() any { return Count([]any{1, "a", 1}) })
	try("unhashable map key", func() any { return Count([]any{1, []int{1}}) })
	try("unhashable struct key", func() any { return Count([]Box{{1}, {map[string]int{}}}) })
}

// Output:
// same type, comparable true
// same type, comparable recovered: <nil>
// different types false
// different types recovered: <nil>
// slice vs slice recovered: runtime error: comparing uncomparable type []int
// map vs map recovered: runtime error: comparing uncomparable type map[int]int
// func vs func recovered: runtime error: comparing uncomparable type func()
// struct holding slice recovered: runtime error: comparing uncomparable type []int
// struct holding int true
// struct holding int recovered: <nil>
// array of any recovered: runtime error: comparing uncomparable type []int
// array of any, differ first false
// array of any, differ first recovered: <nil>
// has true
// has recovered: <nil>
// has, reaches slice recovered: runtime error: comparing uncomparable type []int
// map key 2
// map key recovered: <nil>
// unhashable map key recovered: runtime error: hash of unhashable type []int
// unhashable struct key recovered: runtime error: hash of unhashable type map[string]int
