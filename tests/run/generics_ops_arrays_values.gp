// phase: 07

// Arrays and structs with a type parameter keep value semantics: copies on assignment, call and return.
package main

import "fmt"

type Pt struct{ X, Y int }
type Grid [2][2]int

func Rotate[T any](a [3]T) [3]T {
	a[0], a[1], a[2] = a[1], a[2], a[0]
	return a
}

func Fill[T any](v T) (a [4]T) {
	for i := range a {
		a[i] = v
	}
	return
}

func Mut[T any](a [2]T, v T) [2]T {
	a[0] = v
	return a
}

func Eq[T comparable](a, b [3]T) bool { return a == b }

func Lens[A ~[3]int | ~[3]string]() int {
	var z A
	return len(z)
}

func Sum[T ~int | ~float64](a [3]T) (t T) {
	for _, v := range a {
		t += v
	}
	return
}

func Idx[T any](a [3]T, i int) T { return a[i] }

func Ptr[T any](a *[3]T) { a[1] = a[0] }

func StructCopy[T any](v T) (T, T) {
	w := v
	return v, w
}

func main() {
	orig := [3]string{"a", "b", "c"}
	rot := Rotate(orig)
	fmt.Println(orig, rot, Rotate([3]int{1, 2, 3}), Rotate([3]Pt{{1, 1}, {2, 2}, {3, 3}}))
	fmt.Println(Fill(7), Fill("x"), Fill(Pt{1, 2}), Fill([2]bool{true}))
	m := [2]Pt{{1, 1}, {2, 2}}
	m2 := Mut(m, Pt{9, 9})
	fmt.Println(m, m2)
	fmt.Println(Eq([3]int{1, 2, 3}, [3]int{1, 2, 3}), Eq([3]string{"a"}, [3]string{"b"}), Eq([3]Pt{}, [3]Pt{}), Eq([3]any{1, "a", nil}, [3]any{1, "a", nil}))
	fmt.Println(Lens[[3]int](), Lens[[3]string](), Sum([3]int{1, 2, 3}), Sum([3]float64{.5, .25, .125}), Idx([3]Pt{{}, {5, 6}, {}}, 1))
	arr := [3]int{4, 5, 6}
	Ptr(&arr)
	var g Grid
	g2 := g
	g2[1][1] = 5
	fmt.Println(arr, g, g2)
	a, b := StructCopy(Pt{1, 2})
	a.X = 100
	arrs, arrs2 := StructCopy([2]int{1, 2})
	arrs[0] = 50
	fmt.Println(a, b, arrs, arrs2)
	defer func() { fmt.Println("recovered:", recover()) }()
	i := 3
	fmt.Println(Idx(arr, i))
}

// Output:
// [a b c] [b c a] [2 3 1] [{2 2} {3 3} {1 1}]
// [7 7 7 7] [x x x x] [{1 2} {1 2} {1 2} {1 2}] [[true false] [true false] [true false] [true false]]
// [{1 1} {2 2}] [{9 9} {2 2}]
// true false true true
// 3 3 6 0.875 {5 6}
// [4 4 6] [[0 0] [0 0]] [[0 0] [0 5]]
// {100 2} {1 2} [50 2] [1 2]
// recovered: runtime error: index out of range [3] with length 3
