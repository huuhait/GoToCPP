// phase: 07

// Every form of range with a type parameter: slice, string, map, channel, integer, array pointer.
package main

import "fmt"

type Int interface{ ~int }

type Small int

func Count[T Int](n T) []T {
	var out []T
	for i := range n {
		out = append(out, i)
	}
	return out
}

func Runes[S ~string](s S) (out []string) {
	for i, r := range s {
		out = append(out, fmt.Sprint(i, ":", string(r)))
	}
	return
}

func Sum[S ~[]E, E ~int | ~float64](s S) (t E) {
	for _, v := range s {
		t += v
	}
	return
}

func Idx[S ~[]E, E any](s S) (t int) {
	for i := range s {
		t += i
	}
	return
}

func MapSum[M ~map[K]V, K comparable, V ~int](m M) (t V) {
	for _, v := range m {
		t += v
	}
	return
}

func SortedKeys[M ~map[K]V, K ~string, V any](m M) []K {
	var ks []K
	for k := range m {
		ks = append(ks, k)
	}
	for i := range ks {
		for j := i + 1; j < len(ks); j++ {
			if ks[j] < ks[i] {
				ks[i], ks[j] = ks[j], ks[i]
			}
		}
	}
	return ks
}

func PtrRange[P *[3]E, E any](p P) (out []E) {
	for _, v := range p {
		out = append(out, v)
	}
	return
}

func ArrRange[A ~[3]int](a A) (t int) {
	for i, v := range a {
		a[0] = 100
		t += i + v
	}
	return
}

type Word string
type Score map[Word]int

func main() {
	fmt.Println(Count(4), Count(Small(3)), Count(0), Count(-2))
	fmt.Println(Runes("aé日"), Runes(Word("xy")))
	fmt.Println(Sum([]int{1, 2, 3}), Sum([]float64{.5, .25}), Idx([]string{"a", "b", "c"}))
	fmt.Println(MapSum(Score{"a": 1, "b": 2}), SortedKeys(Score{"z": 1, "a": 2, "m": 3}))
	fmt.Println(PtrRange(&[3]string{"p", "q", "r"}), ArrRange([3]int{1, 2, 3}))
}

// Output:
// [0 1 2 3] [0 1 2] [] []
// [0:a 1:é 3:日] [0:x 1:y]
// 6 0.75 3
// 3 [a m z]
// [p q r] 9
