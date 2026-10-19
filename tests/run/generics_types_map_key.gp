// phase: 07

// Instances of generic types as map keys, in comparisons, and as elements of arrays and channels' buffers.
package main

import "fmt"

type Pair[A, B comparable] struct {
	First  A
	Second B
}

type Coord[T ~int | ~float64] struct{ X, Y T }

func main() {
	m := map[Pair[string, int]]string{}
	m[Pair[string, int]{"a", 1}] = "x"
	m[Pair[string, int]{"a", 2}] = "y"
	m[Pair[string, int]{"a", 1}] += "z"
	fmt.Println(len(m), m[Pair[string, int]{"a", 1}], m[Pair[string, int]{"b", 1}] == "")
	seen := map[Coord[int]]bool{{1, 2}: true}
	fmt.Println(seen[Coord[int]{1, 2}], seen[Coord[int]{2, 1}])
	fm := map[Coord[float64]]int{{0.5, 1.5}: 1}
	fm[Coord[float64]{0.5, 1.5}]++
	fmt.Println(fm)
	fmt.Println(Pair[int, int]{1, 2} == Pair[int, int]{1, 2}, Pair[int, int]{1, 2} == Pair[int, int]{2, 1})
	nested := map[Pair[Pair[int, int], string]]int{{Pair[int, int]{1, 2}, "k"}: 5}
	fmt.Println(nested[Pair[Pair[int, int], string]{Pair[int, int]{1, 2}, "k"}])
	var ia, ib any = Coord[int]{1, 1}, Coord[int]{1, 1}
	var fa any = Coord[float64]{1, 1}
	fmt.Println(ia == ib, ia == fa)
	arr := [2]Coord[int]{{1, 2}, {3, 4}}
	arr2 := arr
	arr2[0].X = 9
	fmt.Println(arr == arr2, arr[0], arr2[0])
}

// Output:
// 2 xz true
// true false
// map[{0.5 1.5}:2]
// true false
// 5
// true false
// false {1 2} {9 2}
