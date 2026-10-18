// phase: 07

// Keys and Values of maps, with a generic insertion sort to get a stable output.
package main

import "fmt"

type Ordered interface {
	~int | ~int64 | ~float64 | ~string
}

func Keys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func Values[K comparable, V any](m map[K]V) []V {
	out := make([]V, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func Sort[T Ordered](xs []T) []T {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
	return xs
}

type Point struct{ X, Y int }

func main() {
	ages := map[string]int{"bob": 31, "alice": 29, "carol": 35}
	fmt.Println(Sort(Keys(ages)))
	fmt.Println(Sort(Values(ages)))
	sq := map[int]float64{3: 9, 1: 1, 2: 4}
	fmt.Println(Sort(Keys(sq)), Sort(Values(sq)))
	pts := map[Point]bool{{1, 2}: true, {0, 0}: false}
	fmt.Println(len(Keys(pts)), len(Values(pts)))
	fmt.Println(len(Keys(map[bool][]string{})), Keys(map[string]int(nil)) == nil)
}

// Output:
// [alice bob carol]
// [29 31 35]
// [1 2 3] [1 4 9]
// 2 2
// 0 false
