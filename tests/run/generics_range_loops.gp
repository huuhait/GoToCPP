// phase: 07

// for range over slices, maps, strings, arrays and integers whose types are type parameters.
package main

import "fmt"

func SumIdx[S ~[]E, E ~int | ~float64](s S) (E, int) {
	var sum E
	idx := 0
	for i, v := range s {
		sum += v
		idx += i
	}
	return sum, idx
}

func Count[N ~int](n N) []N {
	var out []N
	for i := range n {
		out = append(out, i)
	}
	return out
}

func CountU[N ~uint8](n N) []N {
	var out []N
	for i := range n {
		out = append(out, i)
	}
	return out
}

func MapSum[M ~map[K]V, K comparable, V ~int](m M) V {
	var s V
	for _, v := range m {
		s += v
	}
	return s
}

func Runes[T ~string](s T) []rune {
	var out []rune
	for _, r := range s {
		out = append(out, r)
	}
	return out
}

func ArrSum[T ~int | ~float64](a [4]T) T {
	var s T
	for _, v := range a {
		s += v
	}
	return s
}

func Indices[T any](s []T) []int {
	var out []int
	for i := range s {
		out = append(out, i)
	}
	return out
}

func Nested[T any](grid [][]T) int {
	n := 0
	for _, row := range grid {
		for range row {
			n++
		}
	}
	return n
}

type Level uint8
type Step int

func main() {
	fmt.Println(SumIdx([]int{10, 20, 30}))
	fmt.Println(SumIdx([]float64{0.5, 0.25}))
	fmt.Println(Count(4), Count(Step(3)), CountU(Level(2)), CountU[uint8](0))
	fmt.Println(MapSum(map[string]int{"a": 1, "b": 2}), MapSum(map[int]Step{1: 200, 2: 100}))
	fmt.Println(Runes("héy"), Runes[string](""))
	fmt.Println(ArrSum([4]int{1, 2, 3, 4}), ArrSum([4]float64{0.5, 0.5, 0.5, 0.5}))
	fmt.Println(Indices([]string{"a", "b", "c"}), Indices[int](nil))
	fmt.Println(Nested([][]int{{1, 2}, {3}, {}}), Nested([][]string(nil)))
}

// Output:
// 60 3
// 0.75 1
// [0 1 2 3] [0 1 2] [0 1] []
// 3 300
// [104 233 121] []
// 10 2
// [0 1 2] []
// 3 0
