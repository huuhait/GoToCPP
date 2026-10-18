// phase: 07

// f(g()) where g returns several values, with generic f or generic g.
package main

import "fmt"

func Two[T any](v T) (T, T) { return v, v }

func Pair[A, B any](a A, b B) (A, B) { return a, b }

func Join[T any](a, b T) []T { return []T{a, b} }

func Show[A, B any](a A, b B) string { return fmt.Sprint(a, "/", b) }

func Split(s string) (string, int) { return s, len(s) }

func Sum[T ~int | ~float64](vs ...T) T {
	var s T
	for _, v := range vs {
		s += v
	}
	return s
}

func Ints() (int, int, int) { return 1, 2, 3 }

func Tuple[T any](v T) (T, bool, error) { return v, true, nil }

func main() {
	fmt.Println(Join(Two(5)))
	fmt.Println(Join(Two("x")))
	fmt.Println(Show(Pair(1, "a")))
	fmt.Println(Show(Split("hello")))
	fmt.Println(Sum(Ints()))
	fmt.Println(Pair(Two(2.5)))
	fmt.Println(Tuple(3))
	fmt.Println(Tuple("t"))
	a, b := Pair(Split("abc"))
	fmt.Println(a, b)
	x, ok, err := Tuple([]int{1})
	fmt.Println(x, ok, err)
}

// Output:
// [5 5]
// [x x]
// 1/a
// hello/5
// 6
// 2.5 2.5
// 3 true <nil>
// t true <nil>
// abc 3
// [1] true <nil>
