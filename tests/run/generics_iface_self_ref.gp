// phase: 07

// Self-referential constraints: T interface{ Less(T) bool }, Equal(T), Next() T, and methods that take other type parameters.
package main

import "fmt"

type Age int
type Ver struct{ Major, Minor int }

func (a Age) Less(b Age) bool { return a < b }
func (v Ver) Less(w Ver) bool {
	if v.Major != w.Major {
		return v.Major < w.Major
	}
	return v.Minor < w.Minor
}
func (v Ver) Next() Ver { return Ver{v.Major, v.Minor + 1} }
func (a Age) Next() Age { return a + 1 }

func Min[T interface{ Less(T) bool }](xs ...T) (m T) {
	for i, x := range xs {
		if i == 0 || x.Less(m) {
			m = x
		}
	}
	return
}

func Sort[T interface{ Less(T) bool }](xs []T) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j].Less(xs[j-1]); j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

func Succ[T interface {
	Less(T) bool
	Next() T
}](x T, n int) []T {
	out := []T{x}
	for range n {
		x = x.Next()
		out = append(out, x)
	}
	return out
}

func IsSorted[T interface{ Less(T) bool }](xs []T) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i].Less(xs[i-1]) {
			return false
		}
	}
	return true
}

func Between[T interface{ Less(T) bool }](x, lo, hi T) bool { return !x.Less(lo) && x.Less(hi) }

func main() {
	fmt.Println(Min(Age(30), 20, 25), Min(Ver{1, 2}, Ver{1, 0}, Ver{0, 9}), Min[Age]())
	vs := []Ver{{2, 0}, {1, 5}, {1, 2}, {3, 1}}
	Sort(vs)
	as := []Age{9, 3, 7}
	Sort(as)
	fmt.Println(vs, as, IsSorted(vs), IsSorted([]Age{2, 1}))
	fmt.Println(Succ(Ver{1, 9}, 3), Succ(Age(5), 2))
	fmt.Println(Between(Age(5), 1, 10), Between(Ver{1, 1}, Ver{1, 1}, Ver{1, 2}), Between(Age(10), 1, 10))
}

// Output:
// 20 {0 9} 0
// [{1 2} {1 5} {2 0} {3 1}] [3 7 9] true false
// [{1 9} {1 10} {1 11} {1 12}] [5 6 7]
// true true false
