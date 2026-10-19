// phase: 08

// comparable with interface type arguments: since Go 1.20 any interface satisfies comparable, and
// == / map keys panic at run time for an uncomparable dynamic type. The panic texts are Go's and
// the comparison of arrays, structs and pointers of interfaces recurses.
package main

import "fmt"

func Eq[T comparable](a, b T) bool { return a == b }

func Index[T comparable](xs []T, x T) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

func Set[T comparable](xs ...T) map[T]bool {
	m := map[T]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func try(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%s: panic: %v\n", name, r)
		}
	}()
	f()
}

type Key struct {
	A any
	N int
}

type Arr [2]any

type Fn func()

func main() {
	fmt.Println(Eq[any](1, 1), Eq[any](1, int64(1)), Eq[any]("a", "a"), Eq[any](nil, nil), Eq[any](1, nil))
	fmt.Println(Eq[any]([2]int{1, 2}, [2]int{1, 2}), Eq[any](struct{ X int }{1}, struct{ X int }{1}))
	try("slice vs slice", func() { fmt.Println(Eq[any]([]int{1}, []int{1})) })
	try("slice vs int", func() { fmt.Println(Eq[any]([]int{1}, 1)) })
	try("map", func() { fmt.Println(Eq[any](map[int]int{}, map[int]int{})) })
	try("func", func() { fmt.Println(Eq[any](func() {}, func() {})) })
	try("struct with slice", func() {
		fmt.Println(Eq[any](struct{ S []int }{}, struct{ S []int }{}))
	})
	try("array of any", func() { fmt.Println(Eq(Arr{1, []int{}}, Arr{1, []int{}})) })
	try("array of any, differs first", func() { fmt.Println(Eq(Arr{1, []int{}}, Arr{2, []int{}})) })
	try("struct key", func() { fmt.Println(Eq(Key{[]int{}, 1}, Key{[]int{}, 1})) })
	try("struct key, N differs", func() { fmt.Println(Eq(Key{[]int{}, 1}, Key{[]int{}, 2})) })
	try("index", func() {
		fmt.Println(Index[any]([]any{1, "a", 2.5}, 2.5), Index[any]([]any{1, []int{}}, 1), Index[any]([]any{[]int{}}, []int{}))
	})
	try("map key slice", func() { fmt.Println(len(Set[any](1, "a", 2.5, 1))); Set[any]([]int{1}) })
	try("map key func", func() { Set[any](Fn(nil)) })
	try("map key struct", func() { Set(Key{map[string]int{}, 1}) })
	try("map key array", func() { Set(Arr{1, 2}, Arr{1, []string{}}) })
	try("fine", func() { fmt.Println(len(Set(Key{1, 1}, Key{1, 1}, Key{"1", 1}, Key{nil, 0}))) })
	try("nil iface", func() { fmt.Println(Eq[error](nil, nil), Eq[fmt.Stringer](nil, nil)) })
	try("ptr", func() { x, y := new([]int), new([]int); fmt.Println(Eq(x, y), Eq(x, x), Eq[any](x, x)) })
	try("chan", func() { c := make(chan int); fmt.Println(Eq(c, c), Eq[any](c, make(chan int))) })
	try("error values", func() { e := fmt.Errorf("e"); fmt.Println(Eq[error](e, e), Eq[error](e, fmt.Errorf("e"))) })
}

// Output:
// true false true true false
// true true
// slice vs slice: panic: runtime error: comparing uncomparable type []int
// false
// map: panic: runtime error: comparing uncomparable type map[int]int
// func: panic: runtime error: comparing uncomparable type func()
// struct with slice: panic: runtime error: comparing uncomparable type struct { S []int }
// array of any: panic: runtime error: comparing uncomparable type []int
// false
// struct key: panic: runtime error: comparing uncomparable type []int
// struct key, N differs: panic: runtime error: comparing uncomparable type []int
// index: panic: runtime error: comparing uncomparable type []int
// 3
// map key slice: panic: runtime error: hash of unhashable type []int
// map key func: panic: runtime error: hash of unhashable type main.Fn
// map key struct: panic: runtime error: hash of unhashable type map[string]int
// map key array: panic: runtime error: hash of unhashable type []string
// 3
// true true
// false true true
// true false
// true false
