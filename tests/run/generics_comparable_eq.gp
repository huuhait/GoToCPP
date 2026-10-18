// phase: 07

// == and != on type parameters constrained by comparable: structs, arrays, pointers, channels, interfaces.
package main

import "fmt"

type Point struct{ X, Y int }

type Key struct {
	name string
	id   [2]int
	p    *int
}

type Color int

func Eq[T comparable](a, b T) bool { return a == b }
func Ne[T comparable](a, b T) bool { return a != b }

func Count[T comparable](xs []T, v T) int {
	n := 0
	for _, x := range xs {
		if x == v {
			n++
		}
	}
	return n
}

func Dedup[T comparable](xs []T) []T {
	seen := map[T]bool{}
	var out []T
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func Index[K comparable, V any](m map[K]V, k K) (V, bool) {
	v, ok := m[k]
	return v, ok
}

func main() {
	n := 1
	fmt.Println(Eq(1, 1), Eq("a", "a"), Eq(1.5, 1.5), Eq(true, false), Eq(Color(1), Color(1)), Eq('x', 'x'))
	fmt.Println(Eq(Point{1, 2}, Point{1, 2}), Eq(Point{1, 2}, Point{2, 1}), Ne(Point{}, Point{}))
	fmt.Println(Eq([3]int{1, 2, 3}, [3]int{1, 2, 3}), Eq([2]string{"a", "b"}, [2]string{"a", "c"}))
	fmt.Println(Eq(Key{"k", [2]int{1, 2}, &n}, Key{"k", [2]int{1, 2}, &n}), Eq(Key{"k", [2]int{1, 2}, &n}, Key{"k", [2]int{1, 2}, nil}))
	fmt.Println(Eq(&n, &n), Eq(&n, new(int)), Eq[*int](nil, nil))
	fmt.Println(Eq[any](1, 1), Eq[any](1, int8(1)), Eq[any]("a", "a"))
	fmt.Println(Eq[error](nil, nil), Eq[fmt.Stringer](nil, nil), Eq[any](Point{1, 1}, Point{1, 1}), Eq[any]([2]int{1, 1}, [2]int{1, 1}))
	fmt.Println(Count([]string{"a", "b", "a"}, "a"), Count([]Point{{1, 1}, {1, 1}, {0, 0}}, Point{1, 1}), Count([]any{1, "1", 1.0, 1}, any(1)))
	fmt.Println(Dedup([]int{3, 1, 3, 2, 1}), Dedup([]Point{{1, 1}, {2, 2}, {1, 1}}), Dedup([]any{1, "a", 1, "a", 2.5}))
	fmt.Println(Index(map[Point]string{{1, 2}: "p"}, Point{1, 2}))
	fmt.Println(Index(map[[2]int]int{}, [2]int{}))
	fmt.Println(Index(map[any]int{1: 10, "1": 20}, any("1")))
	nan := 0.0
	nan = nan / nan
	fmt.Println(Eq(nan, nan), Ne(nan, nan), Count([]float64{nan}, nan))
}

// Output:
// true true true false true true
// true false false
// true false
// true false
// true false true
// true false true
// true true true true
// 2 2 2
// [3 1 2] [{1 1} {2 2}] [1 a 2.5]
// p true
// 0 false
// 20 true
// false true 0
