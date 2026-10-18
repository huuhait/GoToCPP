// phase: 07

// Arrays of type parameter elements ([3]T), arrays as type arguments, and slicing arrays.
package main

import "fmt"

type Vec [3]float64

func Rev3[T any](a [3]T) [3]T {
	a[0], a[2] = a[2], a[0]
	return a
}

func Fill[T any](v T) (a [4]T) {
	for i := range a {
		a[i] = v
	}
	return
}

func Sum[T ~int | ~float64](a [3]T) (s T) {
	for _, v := range a {
		s += v
	}
	return
}

func Ident[T any](v T) T { return v }

func Slice[T any](a *[5]T) []T { return a[1:4] }

func Eq[T comparable](a, b T) bool { return a == b }

func Len[A ~[3]int | ~[3]string](a A) int { return len(a) }

func Copy[T any](a [2]T) [2]T {
	b := a
	b[0] = a[1]
	return b
}

func Grid[T any](v T) [2][2]T { return [2][2]T{{v, v}, {v, v}} }

func main() {
	fmt.Println(Rev3([3]int{1, 2, 3}), Rev3([3]string{"a", "b", "c"}), Rev3([3]bool{true}))
	fmt.Println(Fill(7), Fill("s"), Fill([2]int{1, 2}), len(Fill(struct{}{})))
	fmt.Println(Sum([3]int{1, 2, 3}), Sum([3]float64{0.5, 0.5, 1}), Sum(Vec{1, 2, 3}))
	fmt.Println(Ident([2]int{1, 2}), Ident(Vec{}), Ident([0]int{}), Ident([2][2]int{{1, 2}, {3, 4}}))
	a := [5]int{1, 2, 3, 4, 5}
	s := Slice(&a)
	s[0] = 20
	fmt.Println(s, a, len(Slice(&[5]string{})))
	fmt.Println(Eq([2]int{1, 2}, [2]int{1, 2}), Eq(Vec{1}, Vec{2}), Eq([1]any{1}, [1]any{1}))
	fmt.Println(Len([3]int{}), Len([3]string{}))
	fmt.Println(Copy([2]int{1, 2}), Copy([2]string{"a", "b"}), Grid(1), Grid("z"))
	fmt.Printf("%T %T %T\n", Rev3([3]int{}), Fill(Vec{}), Grid(1.5))
}

// Output:
// [3 2 1] [c b a] [false false true]
// [7 7 7 7] [s s s s] [[1 2] [1 2] [1 2] [1 2]] 4
// 6 2 6
// [1 2] [0 0 0] [] [[1 2] [3 4]]
// [20 3 4] [1 20 3 4 5] 3
// true false true
// 3 3
// [2 2] [b b] [[1 1] [1 1]] [[z z] [z z]]
// [3]int [4]main.Vec [2][2]float64
