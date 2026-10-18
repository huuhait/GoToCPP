// phase: 07

// Unnamed types as type arguments: slices, maps, arrays, pointers, funcs, structs, nested.
package main

import "fmt"

func Id[T any](v T) T { return v }

func Show[T any](v T) string { return fmt.Sprintf("%T=%v", v, v) }

func Slice[T any](n int, v T) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func Keys[K comparable, V any](m map[K]V) int { return len(m) }

func Call[T, U any](f func(T) U, v T) U { return f(v) }

func Both[T any](a, b T) []T { return []T{a, b} }

func main() {
	fmt.Println(Show([]int{1}), Show(map[string]bool{"a": true}), Show([2]string{"x", "y"}), Show([][]int{{1}, {2}}))
	fmt.Println(Show(struct{ A, B int }{1, 2}), Show([]struct{ N string }{{"n"}}), Show(map[string]struct{}{"k": {}}))
	fmt.Println(Show(map[[2]int][]string{{1, 2}: {"a"}}), Show([]map[string]int{{"a": 1}}), Show(struct{ M map[int]int }{}))
	n := 3
	fmt.Println(Show(&n) != "", Show((*[]int)(nil)), Show(func() {}) != "")
	fmt.Println(Slice(2, []int{1}), Slice(2, struct{ X int }{9}), Slice(1, map[string]int{"z": 0}), Slice(2, [1]bool{true}))
	fmt.Println(Keys(map[struct{ a int }]string{{1}: "x"}), Keys(map[[2]string]int{}), Keys(map[any]int{1: 1, "a": 2}))
	fmt.Println(Call(func(s []int) int { return len(s) }, []int{1, 2, 3}), Call(func(m map[string]int) []string { return nil }, nil) == nil)
	fmt.Println(Call(func(p struct{ A int }) int { return p.A * 2 }, struct{ A int }{21}))
	fmt.Println(Both([]int{1}, []int{2}), Both(struct{ A string }{"a"}, struct{ A string }{"b"}), Both(map[int]int{1: 1}, nil))
	f := Id(func(i int) int { return i + 1 })
	g := Id[func(string) string](func(s string) string { return s + "!" })
	fmt.Println(f(1), g("hi"), Id(Id(Id([]string{"deep"}))))
}

// Output:
// []int=[1] map[string]bool=map[a:true] [2]string=[x y] [][]int=[[1] [2]]
// struct { A int; B int }={1 2} []struct { N string }=[{n}] map[string]struct {}=map[k:{}]
// map[[2]int][]string=map[[1 2]:[a]] []map[string]int=[map[a:1]] struct { M map[int]int }={map[]}
// true *[]int=<nil> true
// [[1] [1]] [{9} {9}] [map[z:0]] [[true] [true]]
// 1 0 2
// 3 true
// 42
// [[1] [2]] [{a} {b}] [map[1:1] map[]]
// 2 hi! [deep]
