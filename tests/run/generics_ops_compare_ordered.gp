// phase: 07

// Comparison operators on type parameters: integers, floats with NaN, strings, defined types.
package main

import "fmt"

type Ordered interface {
	~int | ~int8 | ~uint8 | ~int64 | ~uint64 | ~float32 | ~float64 | ~string
}

type Grade uint8
type Name string

func Cmp[T Ordered](a, b T) []bool {
	return []bool{a == b, a != b, a < b, a <= b, a > b, a >= b}
}

func Less[T Ordered](a, b T) bool { return a < b }

func Sign[T Ordered](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func BinarySearch[S ~[]E, E Ordered](s S, x E) int {
	lo, hi := 0, len(s)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if s[m] < x {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo
}

func IsSorted[S ~[]E, E Ordered](s S) bool {
	for i := 1; i < len(s); i++ {
		if s[i] < s[i-1] {
			return false
		}
	}
	return true
}

func zero() float64 { return 0 }
func nan() float64  { z := zero(); return z / z }
func inf() float64  { return 1 / zero() }
func negz() float64 { return -zero() }

func main() {
	fmt.Println(Cmp(1, 2), Cmp(int8(-128), 127), Cmp(uint8(200), 100), Cmp(uint64(1<<63), 1))
	fmt.Println(Cmp(1.5, 1.5), Cmp(nan(), 1), Cmp(nan(), nan()), Cmp(float32(0.1), 0.1))
	fmt.Println(Cmp("a", "b"), Cmp("é", "z"), Cmp("", "a"), Cmp(Name("abc"), "abd"), Cmp(Grade(3), 3))
	fmt.Println(Cmp(0.0, negz()), Cmp(inf(), 1.79769313486231570814527423731704356798070e+308), Cmp(-inf(), -1.79769313486231570814527423731704356798070e+308))
	fmt.Println(Sign(1, 2), Sign("b", "a"), Sign(nan(), 1), Sign(Grade(2), 2))
	s := []int64{1, 3, 5, 7}
	fmt.Println(BinarySearch(s, 5), BinarySearch(s, 4), BinarySearch(s, 100), BinarySearch([]string{"a", "c"}, "b"), IsSorted(s), IsSorted([]float64{1, nan(), 0}), IsSorted([]Name{"a", "b"}))
	fmt.Println(Less(Grade(1), Grade(2)), Less(Name("b"), "a"), Less(float32(1), 2))
}

// Output:
// [false true true true false false] [false true true true false false] [false true false false true true] [false true false false true true]
// [true false false true false true] [false true false false false false] [false true false false false false] [true false false true false true]
// [false true true true false false] [false true false false true true] [false true true true false false] [false true true true false false] [true false false true false true]
// [true false false true false true] [false true false false true true] [false true true true false false]
// -1 1 0 0
// 2 2 4 1 true true true
// true false true
